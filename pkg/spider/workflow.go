package spider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type Workflow struct {
	messenger WorkflowMessengerAdapter
	storage   WorkflowStorageAdapter
}

func InitWorkflow(
	messenger WorkflowMessengerAdapter,
	storage WorkflowStorageAdapter,
) *Workflow {
	return &Workflow{
		messenger,
		storage,
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
		messenger,
		storage,
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
t					slog.String("error", err.Error()),
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

				err = w.messenger.SendInputMessage(ctx, InputMessage{
					SessionID:  sessionID,
					TaskID:     nextTaskID,
					TenantID:   dep.TenantID,
					WorkflowID: dep.WorkflowID,
					// TODO
					// WorkflowActionID: dep.ID,
					Key:      dep.Key,
					ActionID: dep.ActionID,
					Values:   string(nextInputb),
				})

				if err != nil {
					slog.Error("sent input message failed", slog.Any("error", err.Error()))
					return err
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
t					slog.String("error", err.Error()),
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

				err = w.messenger.SendInputMessage(ctx, InputMessage{
					SessionID:  m.SessionID,
					TaskID:     nextTaskID,
					TenantID:   dep.TenantID,
					WorkflowID: dep.WorkflowID,
					// TODO
					// WorkflowActionID: dep.ID,
					Key:      dep.Key,
					ActionID: dep.ActionID,
					Values:   string(nextInputb),
				})

				if err != nil {
					slog.Error("sent input message failed", slog.Any("error", err.Error()))
					return err
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
