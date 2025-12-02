package spider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Rocket-Innovation/mca-engine-sdk/pkg/events"
	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// EventPublisher interface for publishing workflow events
type EventPublisher interface {
	PublishWorkflowStarted(ctx context.Context, tenantID, workflowID, workflowName, sessionID string, recipientID string, recipientType events.RecipientType, actionKey, actionID string, payload map[string]interface{}) error
	PublishWorkflowCompleted(ctx context.Context, tenantID, workflowID, workflowName, sessionID string, recipientID string, recipientType events.RecipientType) error
	PublishEntered(ctx context.Context, tenantID, workflowID, workflowName, sessionID, taskID string, recipientID string, recipientType events.RecipientType, nodeID, nodeName string, actionKey, actionID, actionLabel string) error
	PublishExitedSuccess(ctx context.Context, tenantID, workflowID, workflowName, sessionID, taskID string, recipientID string, recipientType events.RecipientType, nodeID, nodeName string, actionKey, actionID, actionLabel string, payload map[string]interface{}) error
	PublishExitedFailed(ctx context.Context, tenantID, workflowID, workflowName, sessionID, taskID string, recipientID string, recipientType events.RecipientType, nodeID, nodeName string, actionKey, actionID, actionLabel string, errorMessage string) error
	Close() error
}

type Workflow struct {
	messenger WorkflowMessengerAdapter
	storage   WorkflowStorageAdapter
	publisher EventPublisher
}

func InitWorkflow(
	messenger WorkflowMessengerAdapter,
	storage WorkflowStorageAdapter,
) *Workflow {
	return &Workflow{
		messenger: messenger,
		storage:   storage,
	}
}

// InitWorkflowWithPublisher creates a workflow with event publisher for reporting
func InitWorkflowWithPublisher(
	messenger WorkflowMessengerAdapter,
	storage WorkflowStorageAdapter,
	publisher EventPublisher,
) *Workflow {
	return &Workflow{
		messenger: messenger,
		storage:   storage,
		publisher: publisher,
	}
}

func InitDefaultWorkflow(
	ctx context.Context,
) (*Workflow, error) {
	messenger, err := InitNATSWorkflowMessengerAdapter(ctx, InitNATSWorkflowMessengerAdapterOpt{
		BetaAutoSetupNATS: true,
	})

	if err != nil {
		return nil, err
	}

	storage, err := InitMongodDBWorkflowStorageAdapter(ctx, InitMongodDBWorkflowStorageAdapterOpt{
		BetaAutoSetupSchema: true,
	})

	if err != nil {
		return nil, err
	}

	return &Workflow{
		messenger: messenger,
		storage:   storage,
	}, nil
}

func (w *Workflow) Messenger() WorkflowMessengerAdapter {
	return w.messenger
}

func (w *Workflow) Storage() WorkflowStorageAdapter {
	return w.storage
}

func (w *Workflow) Run(ctx context.Context) error {

	eg := errgroup.Group{}

	eg.Go(func() error {
		return w.listenTriggerMessages(ctx)
	})

	eg.Go(func() error {
		return w.listenOutputMessages(ctx)
	})

	err := eg.Wait()

	if err != nil {
		return err
	}

	return nil
}

func (w *Workflow) listenTriggerMessages(ctx context.Context) error {

	err := w.messenger.ListenTriggerMessages(ctx, func(c TriggerMessageContext, m TriggerMessage) error {

		// Check workflow status before processing trigger
		workflow, err := w.storage.GetFlow(c.Context, m.TenantID, m.WorkflowID)

		if err != nil {
			slog.Error(
				"GetFlow failed",
				slog.Any("error", err.Error()),
				slog.Any("workflow_id", m.WorkflowID),
			)
			return err
		}

		// Skip if workflow is not active
		if workflow.Status != FlowStatusActive {
			slog.Info(
				"workflow not active, skipping trigger",
				slog.Any("workflow_id", m.WorkflowID),
				slog.Any("status", workflow.Status),
			)
			return nil
		}

		workflowAction, err := w.storage.QueryWorkflowAction(c.Context, m.TenantID, m.WorkflowID, m.Key)

		if err != nil {
			slog.Error(
				"QueryWorkflowAction failed",
				slog.Any("error", err.Error()),
				slog.Any("workflow_id", m.WorkflowID),
				slog.Any("key", m.Key),
			)

			return err
		}

		if workflowAction.Disabled {
			return nil
		}

		wvalues := map[string]interface{}{}

		err = json.Unmarshal([]byte(m.Values), &wvalues)

		if err != nil {
			slog.Error("unmarshal value failed", slog.Any("error", err.Error()))
			return err
		}

		sessionUUID, err := uuid.NewV7()

		if err != nil {
			return err
		}

		sessionID := sessionUUID.String()

		nextContextVal := map[string]map[string]interface{}{}

		nextContextVal[m.Key] = map[string]interface{}{
			"output": wvalues,
		}

		nextContextVal["$trigger"] = nextContextVal[m.Key]

		// Extract recipient info from trigger values for event tracking
		recipientID, recipientType := extractRecipientInfo(wvalues)

		// Publish workflow_started event
		if w.publisher != nil {
			err = w.publisher.PublishWorkflowStarted(
				c.Context,
				m.TenantID,
				m.WorkflowID,
				workflow.Name,
				sessionID,
				recipientID,
				recipientType,
				m.Key,
				m.ActionID,
				wvalues,
			)
			if err != nil {
				slog.Error("failed to publish workflow_started event", slog.String("error", err.Error()))
				// Don't return error - event publishing failure shouldn't block workflow
			}
		}

		deps, err := w.storage.QueryWorkflowActionDependencies(c.Context, m.TenantID, m.WorkflowID, m.Key, m.MetaOutput)

		if err != nil {
			slog.Error("QueryWorkflowActionDependencies failed", slog.Any("error", err.Error()))
			return err
		}

		eg := errgroup.Group{}

		eg.SetLimit(10)

		for _, dep := range deps {
			eg.Go(func() error {

				nextTaskUUID, err := uuid.NewV7()

				if err != nil {
					return err
				}

				nextTaskID := nextTaskUUID.String()

				err = w.storage.CreateSessionContext(ctx, m.WorkflowID, sessionID, nextTaskID, nextContextVal)

				if err != nil {
					slog.Error("CreateSessionContext failed", slog.Any("error", err.Error()))
					return err
				}

				nextInput, err := ex(nextContextVal, dep.Map)

				if err != nil {
					slog.Error("process expression failed - check mapper configuration",
					slog.String("error", err.Error()),
						slog.String("workflow_id", dep.WorkflowID),
						slog.String("session_id", sessionID),
						slog.String("to_key", dep.Key),
						slog.String("action_id", dep.ActionID),
						slog.Any("mapper", dep.Map),
						slog.Any("available_context", nextContextVal),
					)
					return err
				}

				slog.Info("after process expression", slog.Any("nextInput", nextInput))

				nextInputb, err := json.Marshal(nextInput)

				if err != nil {
					slog.Error("marshal next input failed", slog.Any("error", err.Error()))
					return err
				}

				slog.Info("after marshall next input", slog.Any("nextInputb", string(nextInputb)))

				// Extract node information for worker
				nodeID := dep.ID
				nodeName := extractNodeName(&dep)
				actionLabel := nodeName // Same as node name

				err = w.messenger.SendInputMessage(ctx, InputMessage{
					SessionID:   sessionID,
					TaskID:      nextTaskID,
					TenantID:    dep.TenantID,
					WorkflowID:  dep.WorkflowID,
					// TODO
					// WorkflowActionID: dep.ID,
					NodeID:      nodeID,      // NEW
					NodeName:    nodeName,    // NEW
					Key:         dep.Key,
					ActionID:    dep.ActionID,
					ActionLabel: actionLabel, // NEW
					Values:      string(nextInputb),
				})

				if err != nil {
					slog.Error("sent input message failed", slog.Any("error", err.Error()))
					return err
				}

				// Publish entered event
				if w.publisher != nil {
					// Extract node information from workflow action
					nodeID := dep.ID
					nodeName := extractNodeName(&dep)
					actionLabel := nodeName // Same as node name for backward compatibility

					err = w.publisher.PublishEntered(
						ctx,
						dep.TenantID,
						dep.WorkflowID,
						workflow.Name,
						sessionID,
						nextTaskID,
						recipientID,
						recipientType,
						nodeID,
						nodeName,
						dep.Key,
						dep.ActionID,
						actionLabel,
					)
					if err != nil {
						slog.Error("failed to publish entered event", slog.String("error", err.Error()))
					}
				}

				return nil
			})
		}

		err = eg.Wait()

		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (w *Workflow) listenOutputMessages(ctx context.Context) error {

	err := w.messenger.ListenOutputMessages(ctx, func(c OutputMessageContext, m OutputMessage) error {

		// Check workflow status before processing output
		workflow, err := w.storage.GetFlow(c.Context, m.TenantID, m.WorkflowID)

		if err != nil {
			slog.Error(
				"GetFlow failed",
				slog.Any("error", err.Error()),
				slog.Any("workflow_id", m.WorkflowID),
			)
			return err
		}

		// Skip if workflow is not active
		if workflow.Status != FlowStatusActive {
			slog.Info(
				"workflow not active, skipping output",
				slog.Any("workflow_id", m.WorkflowID),
				slog.Any("status", workflow.Status),
			)
			return nil
		}

		workflowAction, err := w.storage.QueryWorkflowAction(c.Context, m.TenantID, m.WorkflowID, m.Key)

		if err != nil {
			slog.Error(
				"QueryWorkflowAction failed",
				slog.Any("error", err.Error()),
				slog.Any("workflow_id", m.WorkflowID),
				slog.Any("key", m.Key),
			)

			return err
		}

		if workflowAction.Disabled {
			return nil
		}

		wvalues := map[string]interface{}{}

		err = json.Unmarshal([]byte(m.Values), &wvalues)

		if err != nil {
			slog.Error("unmarshal value failed", slog.Any("error", err.Error()))
			return err
		}

		wcontext, err := w.storage.GetSessionContext(ctx, m.WorkflowID, m.SessionID, m.TaskID)

		if err != nil {
			slog.Error("GetSessionContext failed", slog.Any("error", err.Error()))
			return err
		}

		nextContextVal := wcontext
		nextContextVal[m.Key] = map[string]interface{}{
			"output": wvalues,
		}

		// Extract recipient info from context for event tracking
		recipientID, recipientType := extractRecipientInfoFromContext(wcontext)

		// Publish exited event (action completed successfully)
		if w.publisher != nil {
			// Extract node information from workflow action
			nodeID := workflowAction.ID
			nodeName := extractNodeName(workflowAction)
			actionLabel := nodeName // Same as node name for backward compatibility

			err = w.publisher.PublishExitedSuccess(
				c.Context,
				m.TenantID,
				m.WorkflowID,
				workflow.Name,
				m.SessionID,
				m.TaskID,
				recipientID,
				recipientType,
				nodeID,
				nodeName,
				m.Key,
				workflowAction.ActionID,
				actionLabel,
				wvalues,
			)
			if err != nil {
				slog.Error("failed to publish exited event", slog.String("error", err.Error()))
			}
		}

		deps, err := w.storage.QueryWorkflowActionDependencies(c.Context, m.TenantID, m.WorkflowID, m.Key, m.MetaOutput)

		if err != nil {
			slog.Error("QueryWorkflowActionDependencies failed", slog.Any("error", err.Error()))
			return err
		}

		err = w.storage.DeleteSessionContext(ctx, m.WorkflowID, m.SessionID, m.TaskID)

		if err != nil {
			slog.Error("DeleteSessionContext failed", slog.Any("error", err.Error()))
			return err
		}

		eg := errgroup.Group{}

		eg.SetLimit(10)

		for _, dep := range deps {
			eg.Go(func() error {

				nextTaskUUID, err := uuid.NewV7()

				if err != nil {
					return err
				}

				nextTaskID := nextTaskUUID.String()

				err = w.storage.CreateSessionContext(ctx, m.WorkflowID, m.SessionID, nextTaskID, nextContextVal)

				if err != nil {
					slog.Error("CreateSessionContext failed", slog.Any("error", err.Error()))
					return err
				}

				nextInput, err := ex(nextContextVal, dep.Map)

				if err != nil {
					slog.Error("ex failed - expression evaluation error",
					slog.String("error", err.Error()),
						slog.String("workflow_id", dep.WorkflowID),
						slog.String("session_id", m.SessionID),
						slog.String("from_key", m.Key),
						slog.String("to_key", dep.Key),
						slog.String("action_id", dep.ActionID),
						slog.Any("mapper", dep.Map),
						slog.Any("available_context", nextContextVal),
					)
					return err
				}

				slog.Info("after process expression", slog.Any("nextInput", nextInput))

				nextInputb, err := json.Marshal(nextInput)

				if err != nil {
					slog.Error("marshal next input failed", slog.Any("error", err.Error()))
					return err
				}

				slog.Info("after marshall next input", slog.Any("nextInputb", string(nextInputb)))

				// Extract node information for worker
				nodeID := dep.ID
				nodeName := extractNodeName(&dep)
				actionLabel := nodeName // Same as node name

				err = w.messenger.SendInputMessage(ctx, InputMessage{
					SessionID:   m.SessionID,
					TaskID:      nextTaskID,
					TenantID:    dep.TenantID,
					WorkflowID:  dep.WorkflowID,
					// TODO
					// WorkflowActionID: dep.ID,
					NodeID:      nodeID,      // NEW
					NodeName:    nodeName,    // NEW
					Key:         dep.Key,
					ActionID:    dep.ActionID,
					ActionLabel: actionLabel, // NEW
					Values:      string(nextInputb),
				})

				if err != nil {
					slog.Error("sent input message failed", slog.Any("error", err.Error()))
					return err
				}

				// Publish entered event for the next action
				if w.publisher != nil {
					// Extract node information from workflow action
					nodeID := dep.ID
					nodeName := extractNodeName(&dep)
					actionLabel := nodeName // Same as node name for backward compatibility

					err = w.publisher.PublishEntered(
						ctx,
						dep.TenantID,
						dep.WorkflowID,
						workflow.Name,
						m.SessionID,
						nextTaskID,
						recipientID,
						recipientType,
						nodeID,
						nodeName,
						dep.Key,
						dep.ActionID,
						actionLabel,
					)
					if err != nil {
						slog.Error("failed to publish entered event", slog.String("error", err.Error()))
					}
				}

				return nil
			})
		}

		err = eg.Wait()

		if err != nil {
			return err
		}

		// If no dependencies, this is a terminal node - workflow is completed
		if len(deps) == 0 && w.publisher != nil {
			err = w.publisher.PublishWorkflowCompleted(
				c.Context,
				m.TenantID,
				m.WorkflowID,
				workflow.Name,
				m.SessionID,
				recipientID,
				recipientType,
			)
			if err != nil {
				slog.Error("failed to publish workflow_completed event", slog.String("error", err.Error()))
			} else {
				slog.Info("workflow completed",
					slog.String("workflow_id", m.WorkflowID),
					slog.String("session_id", m.SessionID),
				)
			}
		}

		return nil
	})

	return err
}

func (w *Workflow) Close(ctx context.Context) error {

	err := w.messenger.Close(ctx)

	if err != nil {
		return err
	}

	err = w.storage.Close(ctx)

	if err != nil {
		return err
	}

	return nil
}

func ex(env map[string]map[string]interface{}, mapping map[string]Mapper) (map[string]interface{}, error) {

	if env == nil {
		env = map[string]map[string]interface{}{}
	}

	env["null"] = nil

	env["builtin"] = map[string]interface{}{
		"string": func(value any) string { return fmt.Sprint(value) },
	}

	output := map[string]interface{}{}

	for k, v := range mapping {

		if len(v.Value) == 0 {
			output[k] = ""
			continue
		}

		if v.Mode == MapperModeFixed {
			output[k] = v.Value
			continue
		}

		expression := v.Value

		slog.Info(
			"executing expression",
			slog.String("expression", expression),
			slog.Any("env", env),
		)

		program, err := expr.Compile(expression, expr.Env(env))

		if err != nil {
			return nil, fmt.Errorf("error on expression %v: %s", expression, err.Error())
		}

		slog.Info("executing program", slog.String("disassemble", program.Disassemble()))

		result, err := expr.Run(program, env)

		if err != nil {
			return nil, fmt.Errorf("error on expression %v: %s", expression, err.Error())
		}

		slog.Info("executed program", slog.String("key", k), slog.Any("result", result))

		output[k] = result
	}

	return output, nil
}

// extractRecipientInfo extracts recipient ID and type from trigger values
// Looks for contact.id/contact.user_id or order.id in the trigger payload
func extractRecipientInfo(values map[string]interface{}) (string, events.RecipientType) {
	// Try to find contact info
	if contact, ok := values["contact"].(map[string]interface{}); ok {
		// Prefer user_id if available (consistent with workflow_recipients)
		if userID, ok := contact["user_id"].(string); ok && userID != "" {
			return userID, events.RecipientTypeContacts
		}
		if id, ok := contact["id"].(string); ok && id != "" {
			return id, events.RecipientTypeContacts
		}
	}

	// Try to find order info
	if order, ok := values["order"].(map[string]interface{}); ok {
		if id, ok := order["id"].(string); ok && id != "" {
			return id, events.RecipientTypeOrders
		}
	}

	return "", ""
}

// extractRecipientInfoFromContext extracts recipient info from session context
// The context has structure like {"$trigger": {"output": {"contact": {...}}}}
func extractRecipientInfoFromContext(ctx map[string]map[string]interface{}) (string, events.RecipientType) {
	// Check $trigger context which contains the original trigger data
	if trigger, ok := ctx["$trigger"]; ok {
		if output, ok := trigger["output"].(map[string]interface{}); ok {
			return extractRecipientInfo(output)
		}
	}

	// Fallback: scan all context keys for contact/order data
	for _, v := range ctx {
		if output, ok := v["output"].(map[string]interface{}); ok {
			if id, rtype := extractRecipientInfo(output); id != "" {
				return id, rtype
			}
		}
	}

	return "", ""
}

// extractNodeName extracts node display name from WorkflowAction
// Priority: Meta["name"] > Config["label"] > Key (fallback)
func extractNodeName(action *WorkflowAction) string {
	// Try Meta["name"] first
	if action.Meta != nil {
		if name, ok := action.Meta["name"]; ok && name != "" {
			return name
		}
	}

	// Try Config["label"] second
	if action.Config != nil {
		if label, ok := action.Config["label"].(string); ok && label != "" {
			return label
		}
	}

	// Fallback to action key
	return action.Key
}
