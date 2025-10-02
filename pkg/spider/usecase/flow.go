package usecase

import (
	"context"

	"github.com/targc/spider-go/pkg/spider"
	"github.com/google/uuid"
)

type CreateFlowRequest struct {
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	TriggerType spider.FlowTriggerType `json:"trigger_type"`
	Meta        map[string]string      `json:"meta,omitempty"`
	Actions     []WorkflowActionInput  `json:"actions"`
	Peers       []PeerInput            `json:"peers"`
}

type WorkflowActionInput struct {
	Key      string                   `json:"key"`
	ActionID string                   `json:"action_id"`
	Config   map[string]string        `json:"config"`
	Mapper   map[string]spider.Mapper `json:"mapper"`
	Meta     map[string]string        `json:"meta,omitempty"`
}

type PeerInput struct {
	ParentKey  string `json:"parent_key"`
	MetaOutput string `json:"meta_output"`
	ChildKey   string `json:"child_key"`
}

type UpdateFlowRequest struct {
	TenantID    string                 `json:"tenant_id"`
	FlowID      string                 `json:"flow_id"`
	Name        string                 `json:"name"`
	TriggerType spider.FlowTriggerType `json:"trigger_type"`
	Meta        map[string]string      `json:"meta,omitempty"`
	Status      spider.FlowStatus      `json:"status"`
}

type FlowResponse struct {
	FlowID   string `json:"flow_id"`
	FlowName string `json:"flow_name"`
}

func (u *Usecase) CreateFlow(ctx context.Context, req *CreateFlowRequest) (*FlowResponse, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	flow, err := u.storage.CreateFlow(ctx, &spider.CreateFlowRequest{
		ID:          id.String(),
		TenantID:    req.TenantID,
		Name:        req.Name,
		TriggerType: req.TriggerType,
		Meta:        req.Meta,
	})

	if err != nil {
		return nil, err
	}

	workflowID := flow.ID

	for _, action := range req.Actions {
		_, err = u.storage.AddAction(ctx, &spider.AddActionRequest{
			TenantID:   req.TenantID,
			WorkflowID: workflowID,
			Key:        action.Key,
			ActionID:   action.ActionID,
			Config:     action.Config,
			Map:        action.Mapper,
			Meta:       action.Meta,
		})

		if err != nil {
			return nil, err
		}
	}

	for _, peer := range req.Peers {
		err = u.storage.AddDep(
			ctx,
			req.TenantID,
			workflowID,
			peer.ParentKey,
			peer.MetaOutput,
			peer.ChildKey,
		)

		if err != nil {
			return nil, err
		}
	}

	return &FlowResponse{
		FlowID:   workflowID,
		FlowName: flow.Name,
	}, nil
}

func (u *Usecase) ListFlows(ctx context.Context, tenantID string, page, pageSize int) (*spider.FlowListResponse, error) {
	return u.storage.ListFlows(ctx, tenantID, page, pageSize)
}

type FlowDetailResponse struct {
	FlowID   string                  `json:"flow_id"`
	FlowName string                  `json:"flow_name"`
	TenantID string                  `json:"tenant_id"`
	Actions  []spider.WorkflowAction `json:"actions"`
}

func (u *Usecase) GetFlow(ctx context.Context, tenantID, flowID string) (*FlowDetailResponse, error) {
	flow, err := u.storage.GetFlow(ctx, tenantID, flowID)
	if err != nil {
		return nil, err
	}

	actions, err := u.storage.GetWorkflowActions(ctx, tenantID, flowID)
	if err != nil {
		return nil, err
	}

	return &FlowDetailResponse{
		FlowID:   flowID,
		FlowName: flow.Name,
		TenantID: tenantID,
		Actions:  actions,
	}, nil
}

func (u *Usecase) UpdateFlow(ctx context.Context, req *UpdateFlowRequest) (*spider.Flow, error) {
	storageReq := &spider.UpdateFlowRequest{
		TenantID:    req.TenantID,
		FlowID:      req.FlowID,
		Name:        req.Name,
		TriggerType: req.TriggerType,
		Meta:        req.Meta,
		Status:      req.Status,
	}
	return u.storage.UpdateFlow(ctx, storageReq)
}

func (u *Usecase) DeleteFlow(ctx context.Context, tenantID, flowID string) error {
	return u.storage.DeleteFlow(ctx, tenantID, flowID)
}