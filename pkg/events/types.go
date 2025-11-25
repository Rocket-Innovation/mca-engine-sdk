package events

import (
	"encoding/json"
	"time"
)

// EventType represents the type of workflow event
// Aligned with REPORT_DB.md: entered / exited pattern
type EventType string

const (
	// Workflow lifecycle events
	EventTypeWorkflowStarted   EventType = "workflow_started"   // User enrolled/started workflow (trigger fired)
	EventTypeWorkflowCompleted EventType = "workflow_completed" // Workflow instance finished successfully
	EventTypeWorkflowFailed    EventType = "workflow_failed"    // Workflow instance terminated with error

	// Node/Action events (entered/exited pattern from REPORT_DB)
	EventTypeEntered EventType = "entered" // User entered a node
	EventTypeExited  EventType = "exited"  // User exited a node (with status: success/failed)
)

// ActionID constants - aligned with mca system
// These are the action_id values used in workflow_actions
const (
	// Triggers
	ActionIDContactEnrollmentTrigger = "contact-enrollment-trigger"
	ActionIDOrderEnrollmentTrigger   = "order-enrollment-trigger"
	ActionIDPointEnrollmentTrigger   = "point-enrollment-trigger"

	// Condition
	ActionIDQuery = "query-action"

	// Notifications
	ActionIDSlack = "slack-action"
	ActionIDEmail = "email-action"
	ActionIDSMS   = "sms-action"
	ActionIDLine  = "line-action"

	// Actions
	ActionIDWebhook = "webhook-action"
	ActionIDTag     = "tag-action"

	// Utility
	ActionIDWait = "wait-action"
	ActionIDLog  = "log-action"
)

// RecipientType for tracking who triggered the workflow
type RecipientType string

const (
	RecipientTypeContacts = "contacts"
	RecipientTypeOrders   = "orders"
)

// WorkflowEventPayload is the message structure for Kafka
// Field names aligned with mca-engine-sdk conventions and REPORT_DB.md
type WorkflowEventPayload struct {
	TenantID      string                 `json:"tenant_id"`
	WorkflowID    string                 `json:"workflow_id"`
	WorkflowName  string                 `json:"workflow_name,omitempty"`
	SessionID     string                 `json:"session_id"`               // Workflow instance ID (execution_id in DB)
	TaskID        string                 `json:"task_id,omitempty"`        // Task ID within session
	RecipientID   string                 `json:"recipient_id,omitempty"`   // Contact/Order ID (user_id)
	RecipientType RecipientType          `json:"recipient_type,omitempty"` // contacts / orders
	EventType     EventType              `json:"event_type"`
	ActionKey     string                 `json:"action_key,omitempty"` // Node key (a1, a2, etc.)
	ActionID      string                 `json:"action_id,omitempty"`  // Action type (query-action, slack-action, etc.)
	Status        string                 `json:"status,omitempty"`     // success / failed (for exited events)
	Payload       map[string]interface{} `json:"payload,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	EventTime     time.Time              `json:"event_time"`
	Timestamp     time.Time              `json:"timestamp"`
}

// ToJSON converts payload to JSON bytes
func (p *WorkflowEventPayload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}
