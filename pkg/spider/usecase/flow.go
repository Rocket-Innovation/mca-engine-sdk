package usecase

import (
	"context"

	"github.com/Rocket-Innovation/mca-engine-sdk/pkg/spider"
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
	NodeName string                   `json:"node_name,omitempty"` // User-defined node name
	Config   map[string]interface{}        `json:"config"`
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
	Actions     []WorkflowActionInput  `json:"actions,omitempty"`
	Peers       []PeerInput            `json:"peers,omitempty"`
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
	FlowID      string                  `json:"flow_id"`
	FlowName    string                  `json:"flow_name"`
	TenantID    string                  `json:"tenant_id"`
	TriggerType spider.FlowTriggerType  `json:"trigger_type"`
	Status      spider.FlowStatus       `json:"status"`
	Version     uint64                  `json:"version"`
	Meta        map[string]string       `json:"meta,omitempty"`
	Actions     []spider.WorkflowAction `json:"actions"`
	Peers       []PeerOutput            `json:"peers"`
}

type PeerOutput struct {
	ParentKey  string `json:"parent_key"`
	MetaOutput string `json:"meta_output"`
	ChildKey   string `json:"child_key"`
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

	peers, err := u.storage.GetWorkflowPeers(ctx, tenantID, flowID)
	if err != nil {
		return nil, err
	}

	// Convert spider.WorkflowPeer to PeerOutput
	peerOutputs := make([]PeerOutput, len(peers))
	for i, peer := range peers {
		peerOutputs[i] = PeerOutput{
			ParentKey:  peer.ParentKey,
			MetaOutput: peer.MetaOutput,
			ChildKey:   peer.ChildKey,
		}
	}

	return &FlowDetailResponse{
		FlowID:      flowID,
		FlowName:    flow.Name,
		TenantID:    tenantID,
		TriggerType: flow.TriggerType,
		Status:      flow.Status,
		Version:     flow.Version,
		Meta:        flow.Meta,
		Actions:     actions,
		Peers:       peerOutputs,
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

	flow, err := u.storage.UpdateFlow(ctx, storageReq)
	if err != nil {
		return nil, err
	}

	if req.Actions != nil {
		err = u.storage.DeleteAllActions(ctx, req.TenantID, req.FlowID)
		if err != nil {
			return nil, err
		}

		for _, action := range req.Actions {
			_, err = u.storage.AddAction(ctx, &spider.AddActionRequest{
				TenantID:   req.TenantID,
				WorkflowID: req.FlowID,
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
	}

	if req.Peers != nil {
		err = u.storage.DeleteAllDeps(ctx, req.TenantID, req.FlowID)
		if err != nil {
			return nil, err
		}

		for _, peer := range req.Peers {
			err = u.storage.AddDep(
				ctx,
				req.TenantID,
				req.FlowID,
				peer.ParentKey,
				peer.MetaOutput,
				peer.ChildKey,
			)

			if err != nil {
				return nil, err
			}
		}
	}

	return flow, nil
}

func (u *Usecase) DeleteFlow(ctx context.Context, tenantID, flowID string) error {
	return u.storage.DeleteFlow(ctx, tenantID, flowID)
}
