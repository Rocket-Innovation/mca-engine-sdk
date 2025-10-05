package usecase

import (
	"context"

	"github.com/Rocket-Innovation/spider-go/pkg/spider"
)

func (u *Usecase) DisableAction(ctx context.Context, tenantID, workflowID, key string) error {
	return u.storage.DisableWorkflowAction(ctx, tenantID, workflowID, key)
}

func (u *Usecase) UpdateAction(ctx context.Context, req *spider.UpdateActionRequest) (*spider.WorkflowAction, error) {
	return u.storage.UpdateAction(ctx, req)
}
