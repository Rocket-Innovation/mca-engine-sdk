package spider

import (
	"context"
	"time"
)

type InputMessageContext struct {
	Context    context.Context
	Timestamp  time.Time
	SendOutput func(metaOutput string, values string) error
}

type InputMessage struct {
	SessionID  string
	TaskID     string
	TenantID   string
	WorkflowID string
	// TODO
	// WorkflowActionID string
	NodeID   string // Node ID from workflow definition
	NodeName string // User-defined node display name
	Key      string
	ActionID string
	Values   string
}

func (m *InputMessage) ToOutputMessage(metaOutput, values string) OutputMessage {
	return OutputMessage{
		SessionID:  m.SessionID,
		TaskID:     m.TaskID,
		TenantID:   m.TenantID,
		WorkflowID: m.WorkflowID,
		// TODO
		// WorkflowActionID: m.WorkflowActionID,
		Key:        m.Key,
		MetaOutput: metaOutput,
		Values:     values,
	}
}

type OutputMessageContext struct {
	Context   context.Context
	Timestamp time.Time
}

type OutputMessage struct {
	SessionID  string
	TaskID     string
	TenantID   string
	WorkflowID string
	// TODO
	// WorkflowActionID string
	Key        string
	ActionID   string
	MetaOutput string
	Values     string
}

type TriggerMessageContext struct {
	Context   context.Context
	Timestamp time.Time
}

type TriggerMessage struct {
	TenantID   string
	WorkflowID string
	// TODO
	// WorkflowActionID string
	Key        string
	ActionID   string
	MetaOutput string
	Values     string
}
