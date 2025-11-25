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

// NotificationChannel represents the notification channel type
type NotificationChannel string

const (
	NotificationChannelLINE  NotificationChannel = "line"
	NotificationChannelSlack NotificationChannel = "slack"
	NotificationChannelEmail NotificationChannel = "email"
	NotificationChannelSMS   NotificationChannel = "sms"
)

// DeliveryStatus represents the notification delivery status
type DeliveryStatus string

const (
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusBounced   DeliveryStatus = "bounced"
)

// WorkflowNotificationPayload is the message structure for notification executions Kafka topic
type WorkflowNotificationPayload struct {
	SessionID      string                 `json:"session_id"`      // Session/execution ID
	WorkflowID       string                 `json:"workflow_id"`       // For lookups
	RecipientID      string                 `json:"recipient_id"`      // For linking
	ActionKey        string                 `json:"action_key"`        // Node key (a1, a2, etc.)
	ActionID         string                 `json:"action_id"`         // line-action, slack-action, etc.
	ActionLabel      string                 `json:"action_label,omitempty"`
	Channel          NotificationChannel    `json:"channel"`           // line, slack, email, sms
	MessageContent   string                 `json:"message_content,omitempty"`
	HasTrackingLink  bool                   `json:"has_tracking_link"`
	DeliveryStatus   DeliveryStatus         `json:"delivery_status"`   // sent, delivered, failed, bounced
	Status           string                 `json:"status"`            // success, failed (execution status)
	ErrorMessage     string                 `json:"error_message,omitempty"`
	ProviderResponse map[string]interface{} `json:"provider_response,omitempty"`
	EventTime        time.Time              `json:"event_time"`
	Timestamp        time.Time              `json:"timestamp"`
}

// ToJSON converts payload to JSON bytes
func (p *WorkflowNotificationPayload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// NotificationPublisher publishes notification execution events to Kafka
type NotificationPublisher struct {
	writer *kafka.Writer
	topic  string
}

// NewNotificationPublisher creates a new Kafka publisher for notification events (no auth)
func NewNotificationPublisher(brokers []string) *NotificationPublisher {
	return NewNotificationPublisherWithAuth(PublisherConfig{
		Brokers: brokers,
	})
}

// NewNotificationPublisherWithAuth creates a new Kafka publisher with SASL authentication
func NewNotificationPublisherWithAuth(config PublisherConfig) *NotificationPublisher {
	topic := GetWorkflowNotificationsTopic()
	log.Printf("[WorkflowNotifications] Publisher initialized for topic: %s", topic)

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
		log.Printf("[WorkflowNotifications] Using SASL PLAIN authentication")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Transport:    transport,
	}

	return &NotificationPublisher{
		writer: writer,
		topic:  topic,
	}
}

// Publish sends a notification event to Kafka
func (p *NotificationPublisher) Publish(ctx context.Context, payload *WorkflowNotificationPayload) error {
	data, err := payload.ToJSON()
	if err != nil {
		log.Printf("[WorkflowNotifications] Failed to marshal payload: %v", err)
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(payload.SessionID),
		Value: data,
	})

	if err != nil {
		log.Printf("[WorkflowNotifications] Failed to publish event: %v", err)
		return err
	}

	log.Printf("[WorkflowNotifications] Published %s notification for execution %s action %s status %s",
		payload.Channel, payload.SessionID, payload.ActionKey, payload.Status)
	return nil
}

// PublishSuccess publishes a successful notification execution
func (p *NotificationPublisher) PublishSuccess(
	ctx context.Context,
	sessionID, workflowID, recipientID string,
	actionKey, actionID string,
	channel NotificationChannel,
	messageContent string,
	hasTrackingLink bool,
) error {
	now := time.Now()
	payload := &WorkflowNotificationPayload{
		SessionID:     sessionID,
		WorkflowID:      workflowID,
		RecipientID:     recipientID,
		ActionKey:       actionKey,
		ActionID:        actionID,
		Channel:         channel,
		MessageContent:  messageContent,
		HasTrackingLink: hasTrackingLink,
		DeliveryStatus:  DeliveryStatusDelivered,
		Status:          "success",
		EventTime:       now,
		Timestamp:       now,
	}
	return p.Publish(ctx, payload)
}

// PublishFailed publishes a failed notification execution
func (p *NotificationPublisher) PublishFailed(
	ctx context.Context,
	sessionID, workflowID, recipientID string,
	actionKey, actionID string,
	channel NotificationChannel,
	errorMessage string,
) error {
	now := time.Now()
	payload := &WorkflowNotificationPayload{
		SessionID:    sessionID,
		WorkflowID:     workflowID,
		RecipientID:    recipientID,
		ActionKey:      actionKey,
		ActionID:       actionID,
		Channel:        channel,
		DeliveryStatus: DeliveryStatusFailed,
		Status:         "failed",
		ErrorMessage:   errorMessage,
		EventTime:      now,
		Timestamp:      now,
	}
	return p.Publish(ctx, payload)
}

// Close closes the Kafka writer
func (p *NotificationPublisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
