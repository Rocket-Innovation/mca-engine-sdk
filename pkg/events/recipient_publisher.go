package events

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// RecipientEventType represents the type of recipient event
type RecipientEventType string

const (
	RecipientEventTypeStarted   RecipientEventType = "started"   // Workflow started for recipient
	RecipientEventTypeCompleted RecipientEventType = "completed" // Workflow completed for recipient
	RecipientEventTypeExited    RecipientEventType = "exited"    // Workflow exited early for recipient
)

// WorkflowRecipientPayload is the message structure for workflow recipients Kafka topic
type WorkflowRecipientPayload struct {
	TenantID      string             `json:"tenant_id"`
	WorkflowID    string             `json:"workflow_id"`
	WorkflowName  string             `json:"workflow_name,omitempty"`
	RecipientID   string             `json:"recipient_id"`
	RecipientType string             `json:"recipient_type"` // "contacts", "orders", "points"
	SessionID   string             `json:"session_id"`   // Session/execution ID
	EventType     RecipientEventType `json:"event_type"`     // started, completed, exited
	ExitReason    string             `json:"exit_reason,omitempty"`
	EventTime     time.Time          `json:"event_time"`
	Timestamp     time.Time          `json:"timestamp"`
}

// ToJSON converts payload to JSON bytes
func (p *WorkflowRecipientPayload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// RecipientPublisher publishes workflow recipient events to Kafka
type RecipientPublisher struct {
	writer *kafka.Writer
	topic  string
}

// NewRecipientPublisher creates a new Kafka publisher for recipient events (no auth)
func NewRecipientPublisher(brokers []string) *RecipientPublisher {
	return NewRecipientPublisherWithAuth(PublisherConfig{
		Brokers: brokers,
	})
}

// NewRecipientPublisherWithAuth creates a new Kafka publisher with SASL authentication
func NewRecipientPublisherWithAuth(config PublisherConfig) *RecipientPublisher {
	topic := GetWorkflowExecutionsTopic()
	log.Printf("[WorkflowExecutions] Publisher initialized for topic: %s", topic)

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
		log.Printf("[WorkflowExecutions] Using SASL PLAIN authentication")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Transport:    transport,
	}

	return &RecipientPublisher{
		writer: writer,
		topic:  topic,
	}
}

// Publish sends a recipient event to Kafka
func (p *RecipientPublisher) Publish(ctx context.Context, payload *WorkflowRecipientPayload) error {
	data, err := payload.ToJSON()
	if err != nil {
		log.Printf("[WorkflowExecutions] Failed to marshal payload: %v", err)
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(payload.SessionID),
		Value: data,
	})

	if err != nil {
		log.Printf("[WorkflowExecutions] Failed to publish event: %v", err)
		return err
	}

	log.Printf("[WorkflowExecutions] Published %s for workflow %s execution %s recipient %s",
		payload.EventType, payload.WorkflowID, payload.SessionID, payload.RecipientID)
	return nil
}

// PublishStarted publishes workflow started event for a recipient
func (p *RecipientPublisher) PublishStarted(
	ctx context.Context,
	tenantID, workflowID, workflowName string,
	recipientID, recipientType, sessionID string,
) error {
	now := time.Now()
	payload := &WorkflowRecipientPayload{
		TenantID:      tenantID,
		WorkflowID:    workflowID,
		WorkflowName:  workflowName,
		RecipientID:   recipientID,
		RecipientType: recipientType,
		SessionID:   sessionID,
		EventType:     RecipientEventTypeStarted,
		EventTime:     now,
		Timestamp:     now,
	}
	return p.Publish(ctx, payload)
}

// PublishCompleted publishes workflow completed event for a recipient
func (p *RecipientPublisher) PublishCompleted(
	ctx context.Context,
	tenantID, workflowID string,
	recipientID, sessionID string,
) error {
	now := time.Now()
	payload := &WorkflowRecipientPayload{
		TenantID:    tenantID,
		WorkflowID:  workflowID,
		RecipientID: recipientID,
		SessionID: sessionID,
		EventType:   RecipientEventTypeCompleted,
		EventTime:   now,
		Timestamp:   now,
	}
	return p.Publish(ctx, payload)
}

// PublishExited publishes workflow exited event for a recipient
func (p *RecipientPublisher) PublishExited(
	ctx context.Context,
	tenantID, workflowID string,
	recipientID, sessionID string,
	exitReason string,
) error {
	now := time.Now()
	payload := &WorkflowRecipientPayload{
		TenantID:    tenantID,
		WorkflowID:  workflowID,
		RecipientID: recipientID,
		SessionID: sessionID,
		EventType:   RecipientEventTypeExited,
		ExitReason:  exitReason,
		EventTime:   now,
		Timestamp:   now,
	}
	return p.Publish(ctx, payload)
}

// Close closes the Kafka writer
func (p *RecipientPublisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
