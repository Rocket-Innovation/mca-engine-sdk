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

// ActionChannel represents the action/notification channel type
type ActionChannel string

const (
	ActionChannelLINE  ActionChannel = "line"
	ActionChannelSlack ActionChannel = "slack"
	ActionChannelEmail ActionChannel = "email"
	ActionChannelSMS   ActionChannel = "sms"
)

// DeliveryStatus represents the delivery status
type DeliveryStatus string

const (
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusBounced   DeliveryStatus = "bounced"
)

// ActionExecutionPayload is the message structure for action executions Kafka topic (mca.workflow.actions.{env})
// Field names aligned with REPORT_DB.md
type ActionExecutionPayload struct {
	TenantID         string                 `json:"tenant_id,omitempty"`
	WorkflowID       string                 `json:"workflow_id"`
	SessionID        string                 `json:"session_id"`
	RecipientID      string                 `json:"recipient_id"`
	NodeID           string                 `json:"node_id,omitempty"`    // NEW - Node ID from workflow definition
	NodeName         string                 `json:"node_name,omitempty"`  // NEW - Node display name
	ActionKey        string                 `json:"action_key"`
	ActionID         string                 `json:"action_id,omitempty"`
	ActionType       ActionChannel          `json:"action_type"`           // line, slack, email, sms (was: channel)
	ActionLabel      string                 `json:"action_label,omitempty"` // For backward compatibility
	MessageContent   string                 `json:"message_content,omitempty"`
	HasTrackingLink  bool                   `json:"has_tracking_link"`
	TrackingLink     string                 `json:"tracking_link,omitempty"`  // Generated tracking URL
	OriginalLink     string                 `json:"original_link,omitempty"`  // Original destination URL
	DeliveryStatus   DeliveryStatus         `json:"delivery_status"`          // pending, sent, delivered, failed, bounced
	ExecutionStatus  string                 `json:"execution_status"`         // success, failed (was: status)
	ErrorMessage     string                 `json:"error_message,omitempty"`
	ProviderResponse map[string]interface{} `json:"provider_response,omitempty"`
	EventTime        time.Time              `json:"event_time"`
	Timestamp        time.Time              `json:"timestamp"`
}

// ToJSON converts payload to JSON bytes
func (p *ActionExecutionPayload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// ActionPublisher publishes action execution events to Kafka (mca.workflow.actions.{env})
type ActionPublisher struct {
	writer *kafka.Writer
	topic  string
}

// NewActionPublisher creates a new Kafka publisher for action events (no auth)
func NewActionPublisher(brokers []string) *ActionPublisher {
	return NewActionPublisherWithAuth(PublisherConfig{
		Brokers: brokers,
	})
}

// NewActionPublisherWithAuth creates a new Kafka publisher with SASL authentication
func NewActionPublisherWithAuth(config PublisherConfig) *ActionPublisher {
	topic := GetWorkflowActionsTopic()
	log.Printf("[WorkflowActions] ActionPublisher initialized for topic: %s", topic)

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
		log.Printf("[WorkflowActions] Using SASL PLAIN authentication")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Transport:    transport,
	}

	return &ActionPublisher{
		writer: writer,
		topic:  topic,
	}
}

// Publish sends an action execution event to Kafka
func (p *ActionPublisher) Publish(ctx context.Context, payload *ActionExecutionPayload) error {
	data, err := payload.ToJSON()
	if err != nil {
		log.Printf("[WorkflowActions] Failed to marshal payload: %v", err)
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(payload.SessionID),
		Value: data,
	})

	if err != nil {
		log.Printf("[WorkflowActions] Failed to publish event: %v", err)
		return err
	}

	log.Printf("[WorkflowActions] Published %s action for session %s action %s status %s",
		payload.ActionType, payload.SessionID, payload.ActionKey, payload.ExecutionStatus)
	return nil
}

// PublishSent publishes an action_sent event (message sent to provider)
func (p *ActionPublisher) PublishSent(
	ctx context.Context,
	tenantID, sessionID, workflowID, recipientID string,
	nodeID, nodeName string,
	actionKey, actionID, actionLabel string,
	actionType ActionChannel,
	messageContent string,
	hasTrackingLink bool,
	success bool, // true = sent successfully, false = send failed
	errorMessage string, // only if success=false
) error {
	now := time.Now()

	deliveryStatus := DeliveryStatusSent
	executionStatus := "success"
	if !success {
		deliveryStatus = DeliveryStatusFailed
		executionStatus = "failed"
	}

	payload := &ActionExecutionPayload{
		TenantID:        tenantID,
		WorkflowID:      workflowID,
		SessionID:       sessionID,
		RecipientID:     recipientID,
		NodeID:          nodeID,
		NodeName:        nodeName,
		ActionKey:       actionKey,
		ActionID:        actionID,
		ActionLabel:     actionLabel,
		ActionType:      actionType,
		MessageContent:  messageContent,
		HasTrackingLink: hasTrackingLink,
		DeliveryStatus:  deliveryStatus,
		ExecutionStatus: executionStatus,
		ErrorMessage:    errorMessage,
		EventTime:       now,
		Timestamp:       now,
	}
	return p.Publish(ctx, payload)
}

// PublishDelivered publishes an action_delivered event (delivery confirmed by provider webhook)
func (p *ActionPublisher) PublishDelivered(
	ctx context.Context,
	tenantID, sessionID, workflowID, recipientID string,
	nodeID, nodeName string,
	actionKey, actionID, actionLabel string,
	actionType ActionChannel,
	providerResponse map[string]interface{},
) error {
	now := time.Now()
	payload := &ActionExecutionPayload{
		TenantID:         tenantID,
		WorkflowID:       workflowID,
		SessionID:        sessionID,
		RecipientID:      recipientID,
		NodeID:           nodeID,
		NodeName:         nodeName,
		ActionKey:        actionKey,
		ActionID:         actionID,
		ActionLabel:      actionLabel,
		ActionType:       actionType,
		DeliveryStatus:   DeliveryStatusDelivered,
		ExecutionStatus:  "success",
		ProviderResponse: providerResponse,
		EventTime:        now,
		Timestamp:        now,
	}
	return p.Publish(ctx, payload)
}

// PublishSuccess publishes a successful action execution
// Deprecated: Use PublishSent for action_sent events and PublishDelivered for action_delivered events
func (p *ActionPublisher) PublishSuccess(
	ctx context.Context,
	tenantID, sessionID, workflowID, recipientID string,
	nodeID, nodeName string, // NEW - from workflow definition
	actionKey, actionID, actionLabel string,
	actionType ActionChannel,
	messageContent string,
	hasTrackingLink bool,
) error {
	now := time.Now()
	payload := &ActionExecutionPayload{
		TenantID:        tenantID,
		WorkflowID:      workflowID,
		SessionID:       sessionID,
		RecipientID:     recipientID,
		NodeID:          nodeID,      // NEW
		NodeName:        nodeName,    // NEW
		ActionKey:       actionKey,
		ActionID:        actionID,
		ActionLabel:     actionLabel, // For backward compatibility
		ActionType:      actionType,
		MessageContent:  messageContent,
		HasTrackingLink: hasTrackingLink,
		DeliveryStatus:  DeliveryStatusDelivered,
		ExecutionStatus: "success",
		EventTime:       now,
		Timestamp:       now,
	}
	return p.Publish(ctx, payload)
}

// PublishFailed publishes a failed action execution
func (p *ActionPublisher) PublishFailed(
	ctx context.Context,
	tenantID, sessionID, workflowID, recipientID string,
	nodeID, nodeName string, // NEW - from workflow definition
	actionKey, actionID, actionLabel string,
	actionType ActionChannel,
	errorMessage string,
) error {
	now := time.Now()
	payload := &ActionExecutionPayload{
		TenantID:        tenantID,
		WorkflowID:      workflowID,
		SessionID:       sessionID,
		RecipientID:     recipientID,
		NodeID:          nodeID,      // NEW
		NodeName:        nodeName,    // NEW
		ActionKey:       actionKey,
		ActionID:        actionID,
		ActionLabel:     actionLabel, // For backward compatibility
		ActionType:      actionType,
		DeliveryStatus:  DeliveryStatusFailed,
		ExecutionStatus: "failed",
		ErrorMessage:    errorMessage,
		EventTime:       now,
		Timestamp:       now,
	}
	return p.Publish(ctx, payload)
}

// PublishLinkOpened publishes a link_opened event when a user clicks a tracking link
func (p *ActionPublisher) PublishLinkOpened(
	ctx context.Context,
	tenantID, sessionID, workflowID, recipientID string,
	nodeID, nodeName string,
	actionKey, actionID, actionLabel string,
	actionType ActionChannel,
	trackingID, originalLink string,
) error {
	now := time.Now()
	payload := &ActionExecutionPayload{
		TenantID:        tenantID,
		WorkflowID:      workflowID,
		SessionID:       sessionID,
		RecipientID:     recipientID,
		NodeID:          nodeID,
		NodeName:        nodeName,
		ActionKey:       actionKey,
		ActionID:        actionID,
		ActionLabel:     actionLabel,
		ActionType:      actionType,
		HasTrackingLink: true,
		TrackingLink:    trackingID, // Store tracking ID
		OriginalLink:    originalLink,
		DeliveryStatus:  DeliveryStatusDelivered,
		ExecutionStatus: "success",
		EventTime:       now,
		Timestamp:       now,
	}
	return p.Publish(ctx, payload)
}

// Close closes the Kafka writer
func (p *ActionPublisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
