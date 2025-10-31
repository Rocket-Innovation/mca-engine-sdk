package spider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type NatsTriggerMessage struct {
	WorkflowID string `json:"workflow_id"`
	TenantID   string `json:"tenant_id"`
	// TODO
	// WorkflowActionID string `json:"workflow_action_id"`
	MetaOutput string `json:"meta_output"`
	Key        string `json:"key"`
	ActionID   string `json:"action_id"`
	Values     string `json:"values"`
}

func (n NatsTriggerMessage) FromTriggerMessage(message TriggerMessage) NatsTriggerMessage {
	return NatsTriggerMessage{
		WorkflowID: message.WorkflowID,
		TenantID:   message.TenantID,
		// TODO:
		// WorkflowActionID: message.WorkflowActionID,
		MetaOutput: message.MetaOutput,
		Key:        message.Key,
		ActionID:   message.ActionID,
		Values:     message.Values,
	}
}

func (n *NatsTriggerMessage) ToTriggerMessage() TriggerMessage {
	return TriggerMessage{
		WorkflowID: n.WorkflowID,
		TenantID:   n.TenantID,
		// TODO
		// WorkflowActionID: b.WorkflowActionID,
		MetaOutput: n.MetaOutput,
		Key:        n.Key,
		ActionID:   n.ActionID,
		Values:     n.Values,
	}
}

type NatsOutputMessage struct {
	SessionID  string `json:"session_id"`
	TaskID     string `json:"task_id"`
	WorkflowID string `json:"workflow_id"`
	TenantID   string `json:"tenant_id"`
	// TODO
	// WorkflowActionID string `json:"workflow_action_id"`
	MetaOutput string `json:"meta_output"`
	Key        string `json:"key"`
	ActionID   string `json:"action_id"`
	Values     string `json:"values"`
}

func (n NatsOutputMessage) FromOutputMessage(message OutputMessage) NatsOutputMessage {
	return NatsOutputMessage{
		SessionID:  message.SessionID,
		TaskID:     message.TaskID,
		WorkflowID: message.WorkflowID,
		TenantID:   message.TenantID,
		// TODO:
		// WorkflowActionID: message.WorkflowActionID,
		MetaOutput: message.MetaOutput,
		Key:        message.Key,
		ActionID:   message.ActionID,
		Values:     message.Values,
	}
}

func (n *NatsOutputMessage) ToOutputMessage() OutputMessage {
	return OutputMessage{
		SessionID:  n.SessionID,
		TaskID:     n.TaskID,
		WorkflowID: n.WorkflowID,
		TenantID:   n.TenantID,
		// TODO
		// WorkflowActionID: b.WorkflowActionID,
		MetaOutput: n.MetaOutput,
		Key:        n.Key,
		ActionID:   n.ActionID,
		Values:     n.Values,
	}
}

type NatsInputMessage struct {
	SessionID  string `json:"session_id"`
	TaskID     string `json:"task_id"`
	WorkflowID string `json:"workflow_id"`
	TenantID   string `json:"tenant_id"`
	// TODO
	// WorkflowActionID string `json:"workflow_action_id"`
	Key      string `json:"key"`
	ActionID string `json:"action_id"`
	Values   string `json:"values"`
}

func (n *NatsInputMessage) ToInputMessage() InputMessage {
	return InputMessage{
		SessionID:  n.SessionID,
		TaskID:     n.TaskID,
		WorkflowID: n.WorkflowID,
		TenantID:   n.TenantID,
		// TODO
		// WorkflowActionID: n.WorkflowActionID,
		Key:      n.Key,
		ActionID: n.ActionID,
		Values:   n.Values,
	}
}

func buildTriggerSubject(prefix string) string {
	return fmt.Sprintf("%s-trigger", prefix)
}

func buildInputSubject(prefix string) string {
	return fmt.Sprintf("%s-input", prefix)
}

func buildOutputSubject(prefix string) string {
	return fmt.Sprintf("%s-output", prefix)
}

func buildWorkflowActionTriggerConsumerID(prefix string) string {
	return fmt.Sprintf("%s-workflow-action-trigger", prefix)
}

func buildWorkflowActionOutputConsumerID(prefix string) string {
	return fmt.Sprintf("%s-workflow-action-output", prefix)
}

func buildWorkerConsumerID(prefix, actionID string) string {
	return fmt.Sprintf("%s-worker-%s", prefix, actionID)
}

func betaCreateJetstream(ctx context.Context, js jetstream.JetStream, stream string) error {
	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        stream,
		Description: "",
		Subjects: []string{
			stream,
		},
		Retention:              jetstream.LimitsPolicy,
		MaxConsumers:           0,
		MaxMsgs:                0,
		MaxBytes:               0,
		Discard:                jetstream.DiscardOld,
		DiscardNewPerSubject:   false,
		MaxAge:                 time.Hour,
		MaxMsgsPerSubject:      0,
		MaxMsgSize:             0,
		Storage:                jetstream.MemoryStorage,
		Replicas:               1,
		NoAck:                  false,
		Duplicates:             time.Minute * 2,
		Placement:              nil,
		Mirror:                 nil,
		Sources:                nil,
		Sealed:                 false,
		DenyDelete:             false,
		DenyPurge:              false,
		AllowRollup:            false,
		Compression:            0,
		FirstSeq:               0,
		SubjectTransform:       nil,
		RePublish:              nil,
		AllowDirect:            false,
		MirrorDirect:           false,
		ConsumerLimits:         jetstream.StreamConsumerLimits{},
		Metadata:               map[string]string{},
		Template:               "",
		AllowMsgTTL:            false,
		SubjectDeleteMarkerTTL: 0,
	})

	if err != nil {
		return err
	}

	slog.Info("nats jetstream created", slog.String("stream", stream))

	return nil
}

func betaCreateConsumer(ctx context.Context, js jetstream.JetStream, stream, consumerID string) error {
	_, err := js.Consumer(ctx, stream, consumerID)

	if err != nil && !errors.Is(err, jetstream.ErrConsumerNotFound) {
		return err
	}

	_, err = js.CreateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Name:               consumerID,
		Durable:            "",
		Description:        "",
		DeliverPolicy:      jetstream.DeliverAllPolicy,
		OptStartSeq:        0,
		OptStartTime:       nil,
		AckPolicy:          jetstream.AckExplicitPolicy,
		AckWait:            0,
		MaxDeliver:         0,
		BackOff:            []time.Duration{},
		FilterSubject:      "",
		ReplayPolicy:       0,
		RateLimit:          0,
		SampleFrequency:    "",
		MaxWaiting:         0,
		MaxAckPending:      0,
		HeadersOnly:        false,
		MaxRequestBatch:    0,
		MaxRequestExpires:  0,
		MaxRequestMaxBytes: 0,
		InactiveThreshold:  0,
		Replicas:           0,
		MemoryStorage:      false,
		FilterSubjects: []string{
			stream,
		},
		Metadata:       map[string]string{},
		PauseUntil:     &time.Time{},
		PriorityPolicy: 0,
		PinnedTTL:      0,
		PriorityGroups: []string{},
	})

	if err != nil {
		return err
	}

	slog.Info("nats consumer created", slog.String("consumer_id", consumerID))

	return nil
}
