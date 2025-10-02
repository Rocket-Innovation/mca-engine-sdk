package apis

import (
	"github.com/targc/spider-go/pkg/spider"
	"github.com/targc/spider-go/pkg/spider/usecase"
)

type Handler struct {
	usecase *usecase.Usecase
}

func NewHandler(usecase *usecase.Usecase) *Handler {
	return &Handler{
		usecase: usecase,
	}
}

type WorkflowAction struct {
	Key      string                   `json:"key" example:"a1"`
	ActionID string                   `json:"action_id" example:"slack-action"`
	Config   map[string]string        `json:"config"`
	Mapper   map[string]spider.Mapper `json:"mapper"`
	Meta     map[string]string        `json:"meta,omitempty"`
}

type Peer struct {
	ParentKey  string `json:"parent_key"`
	MetaOutput string `json:"meta_output"`
	ChildKey   string `json:"child_key"`
}

// CreateFlowPayload represents the request body for creating a flow
type CreateFlowPayload struct {
	Name        string                 `json:"name" example:"My Workflow"`
	TriggerType spider.FlowTriggerType `json:"trigger_type" example:"event"`
	Meta        map[string]string      `json:"meta,omitempty"`
	Actions     []WorkflowAction       `json:"actions"`
	Peers       []Peer                 `json:"peers"`
}

// UpdateFlowPayload represents the request body for updating a flow
type UpdateFlowPayload struct {
	Name        string                 `json:"name" example:"Updated Workflow"`
	TriggerType spider.FlowTriggerType `json:"trigger_type" example:"schedule"`
	Meta        map[string]string      `json:"meta,omitempty"`
	Status      spider.FlowStatus      `json:"status" example:"active"`
}

// UpdateActionPayload represents the request body for updating an action
type UpdateActionPayload struct {
	Config map[string]string        `json:"config"`
	Mapper map[string]spider.Mapper `json:"mapper"`
	Meta   map[string]string        `json:"meta,omitempty"`
}
