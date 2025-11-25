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
type Publisher struct {
	writer *kafka.Writer
	topic  string
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
	topic := GetWorkflowEventsTopic()
	log.Printf("[WorkflowEvents] Publisher initialized for topic: %s", topic)

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

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond, // Low latency
		Transport:    transport,
	}

	return &Publisher{
		writer: writer,
		topic:  topic,
	}
}

// Publish sends an event to Kafka
func (p *Publisher) Publish(ctx context.Context, event *WorkflowEventPayload) error {
	data, err := event.ToJSON()
	if err != nil {
		log.Printf("[WorkflowEvents] Failed to marshal event: %v", err)
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.SessionID), // Partition by session (instance)
		Value: data,
	})

	if err != nil {
		log.Printf("[WorkflowEvents] Failed to publish event: %v", err)
		return err
	}

	log.Printf("[WorkflowEvents] Published %s for workflow %s session %s action %s",
		event.EventType, event.WorkflowID, event.SessionID, event.ActionKey)
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

// Close closes the Kafka writer
func (p *Publisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
