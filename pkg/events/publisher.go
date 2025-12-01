package events

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Publisher publishes workflow events to Kafka
// It uses separate writers for different event types:
// - nodesWriter: for node entered/exited events -> mca.workflow.nodes.{env}
// - executionsWriter: for workflow started/completed/exited events -> mca.workflow.executions.{env}
type Publisher struct {
	nodesWriter      *kafka.Writer
	executionsWriter *kafka.Writer
	nodesTopic       string
	executionsTopic  string
}

// PublisherConfig holds configuration for the Kafka publisher
type PublisherConfig struct {
	Brokers  []string
	Username string
	Password string
}

// NewPublisher creates a new Kafka publisher without authentication (for local/testing)
func NewPublisher(brokers []string) *Publisher {
	return NewPublisherWithAuth(PublisherConfig{
		Brokers: brokers,
	})
}

// NewPublisherWithAuth creates a new Kafka publisher with SASL authentication
func NewPublisherWithAuth(config PublisherConfig) *Publisher {
	nodesTopic := GetWorkflowNodesTopic()
	executionsTopic := GetWorkflowExecutionsTopic()
	log.Printf("[WorkflowEvents] Publisher initialized for topics: nodes=%s, executions=%s", nodesTopic, executionsTopic)

	// Configure transport with TLS and SASL if credentials provided
	var transport *kafka.Transport
	if config.Username != "" && config.Password != "" {
		mechanism := plain.Mechanism{
			Username: config.Username,
			Password: config.Password,
		}
		transport = &kafka.Transport{
			SASL: mechanism,
			TLS:  &tls.Config{},
		}
		log.Printf("[WorkflowEvents] Using SASL PLAIN authentication")
	}

	nodesWriter := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        nodesTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond, // Low latency
		Transport:    transport,
	}

	executionsWriter := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        executionsTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond, // Low latency
		Transport:    transport,
	}

	return &Publisher{
		nodesWriter:      nodesWriter,
		executionsWriter: executionsWriter,
		nodesTopic:       nodesTopic,
		executionsTopic:  executionsTopic,
	}
}

// Publish sends an event to the appropriate Kafka topic based on event type
func (p *Publisher) Publish(ctx context.Context, event *WorkflowEventPayload) error {
	data, err := event.ToJSON()
	if err != nil {
		log.Printf("[WorkflowEvents] Failed to marshal event: %v", err)
		return err
	}

	// Route to the appropriate writer based on event type
	var writer *kafka.Writer
	var topicName string
	switch event.EventType {
	case EventTypeWorkflowStarted, EventTypeWorkflowCompleted, EventTypeWorkflowFailed:
		// Workflow-level events go to executions topic
		writer = p.executionsWriter
		topicName = p.executionsTopic
	case EventTypeEntered, EventTypeExited:
		// Node-level events go to nodes topic
		writer = p.nodesWriter
		topicName = p.nodesTopic
	default:
		// Default to nodes topic for unknown event types
		writer = p.nodesWriter
		topicName = p.nodesTopic
	}

	err = writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.SessionID), // Partition by session (instance)
		Value: data,
	})

	if err != nil {
		log.Printf("[WorkflowEvents] Failed to publish event to %s: %v", topicName, err)
		return err
	}

	log.Printf("[WorkflowEvents] Published %s to %s for workflow %s session %s action %s",
		event.EventType, topicName, event.WorkflowID, event.SessionID, event.ActionKey)
	return nil
}

// PublishWorkflowStarted publishes workflow started event (when user enters workflow at trigger node)
func (p *Publisher) PublishWorkflowStarted(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID string,
	recipientID string, recipientType RecipientType,
	actionKey, actionID string,
	payload map[string]interface{},
) error {
	now := time.Now()
	event := &WorkflowEventPayload{
		TenantID:      tenantID,
		WorkflowID:    workflowID,
		WorkflowName:  workflowName,
		SessionID:     sessionID,
		RecipientID:   recipientID,
		RecipientType: recipientType,
		EventType:     EventTypeWorkflowStarted,
		ActionKey:     actionKey,
		ActionID:      actionID,
		Payload:       payload,
		EventTime:     now,
		Timestamp:     now,
	}
	return p.Publish(ctx, event)
}

// PublishEntered publishes event when user enters a node
func (p *Publisher) PublishEntered(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID, taskID string,
	recipientID string, recipientType RecipientType,
	actionKey, actionID string,
) error {
	now := time.Now()
	event := &WorkflowEventPayload{
		TenantID:      tenantID,
		WorkflowID:    workflowID,
		WorkflowName:  workflowName,
		SessionID:     sessionID,
		TaskID:        taskID,
		RecipientID:   recipientID,
		RecipientType: recipientType,
		EventType:     EventTypeEntered,
		ActionKey:     actionKey,
		ActionID:      actionID,
		EventTime:     now,
		Timestamp:     now,
	}
	return p.Publish(ctx, event)
}

// PublishExited publishes event when user exits a node (success or failed)
func (p *Publisher) PublishExited(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID, taskID string,
	recipientID string, recipientType RecipientType,
	actionKey, actionID string,
	status string, // "success" or "failed"
	errorMessage string,
	payload map[string]interface{},
) error {
	now := time.Now()
	event := &WorkflowEventPayload{
		TenantID:      tenantID,
		WorkflowID:    workflowID,
		WorkflowName:  workflowName,
		SessionID:     sessionID,
		TaskID:        taskID,
		RecipientID:   recipientID,
		RecipientType: recipientType,
		EventType:     EventTypeExited,
		ActionKey:     actionKey,
		ActionID:      actionID,
		Status:        status,
		ErrorMessage:  errorMessage,
		Payload:       payload,
		EventTime:     now,
		Timestamp:     now,
	}
	return p.Publish(ctx, event)
}

// PublishExitedSuccess is a convenience method for successful exit
func (p *Publisher) PublishExitedSuccess(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID, taskID string,
	recipientID string, recipientType RecipientType,
	actionKey, actionID string,
	payload map[string]interface{},
) error {
	return p.PublishExited(ctx, tenantID, workflowID, workflowName, sessionID, taskID,
		recipientID, recipientType, actionKey, actionID, "success", "", payload)
}

// PublishExitedFailed is a convenience method for failed exit
func (p *Publisher) PublishExitedFailed(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID, taskID string,
	recipientID string, recipientType RecipientType,
	actionKey, actionID string,
	errorMessage string,
) error {
	return p.PublishExited(ctx, tenantID, workflowID, workflowName, sessionID, taskID,
		recipientID, recipientType, actionKey, actionID, "failed", errorMessage, nil)
}

// PublishWorkflowCompleted publishes workflow completed event
func (p *Publisher) PublishWorkflowCompleted(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID string,
	recipientID string, recipientType RecipientType,
) error {
	now := time.Now()
	event := &WorkflowEventPayload{
		TenantID:      tenantID,
		WorkflowID:    workflowID,
		WorkflowName:  workflowName,
		SessionID:     sessionID,
		RecipientID:   recipientID,
		RecipientType: recipientType,
		EventType:     EventTypeWorkflowCompleted,
		EventTime:     now,
		Timestamp:     now,
	}
	return p.Publish(ctx, event)
}

// PublishWorkflowFailed publishes workflow failed event
func (p *Publisher) PublishWorkflowFailed(
	ctx context.Context,
	tenantID, workflowID, workflowName, sessionID string,
	recipientID string, recipientType RecipientType,
	errorMessage string,
) error {
	now := time.Now()
	event := &WorkflowEventPayload{
		TenantID:      tenantID,
		WorkflowID:    workflowID,
		WorkflowName:  workflowName,
		SessionID:     sessionID,
		RecipientID:   recipientID,
		RecipientType: recipientType,
		EventType:     EventTypeWorkflowFailed,
		ErrorMessage:  errorMessage,
		EventTime:     now,
		Timestamp:     now,
	}
	return p.Publish(ctx, event)
}

// Close closes all Kafka writers
func (p *Publisher) Close() error {
	var errs []error
	if p.nodesWriter != nil {
		if err := p.nodesWriter.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.executionsWriter != nil {
		if err := p.executionsWriter.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
