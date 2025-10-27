package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/Rocket-Innovation/mca-engine-sdk/pkg/spider"
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())

	workflow, err := spider.InitDefaultWorkflow(ctx)

	if err != nil {
		panic(err)
	}

	storage := workflow.Storage()

	tenantID := os.Getenv("TENANT_ID")
	if tenantID == "" {
		panic("TENANT_ID environment variable is required")
	}
	workflowID := "wa"

	_, err = storage.AddAction(ctx, &spider.AddActionRequest{
		TenantID:   tenantID,
		WorkflowID: workflowID,
		Key:        "a1",
		ActionID:   "test-action-a",
		Config: map[string]interface{}{
			"test": "a",
		},
		Map:  nil,
		Meta: nil,
	})

	if err != nil {
		panic(err)
	}

	_, err = storage.AddAction(ctx, &spider.AddActionRequest{
		TenantID:   tenantID,
		WorkflowID: workflowID,
		Key:        "a2",
		ActionID:   "test-action-b",
		Config: map[string]interface{}{
			"test": "b",
		},
		Map: map[string]spider.Mapper{
			"value": {
				Mode:  spider.MapperModeExpression,
				Value: "a1.output.value + '_updatedx1'",
			},
		},
		Meta: map[string]string{
			"description": "Second action in the workflow",
		},
	})

	if err != nil {
		panic(err)
	}

	_, err = storage.AddAction(ctx, &spider.AddActionRequest{
		TenantID:   tenantID,
		WorkflowID: workflowID,
		Key:        "a3",
		ActionID:   "test-action-b",
		Config: map[string]interface{}{
			"test": "c",
		},
		Map: map[string]spider.Mapper{
			"value": {
				Mode:  spider.MapperModeExpression,
				Value: "a2.output.value + '_updatedx2'",
			},
		},
		Meta: nil,
	})

	if err != nil {
		panic(err)
	}

	err = storage.AddDep(ctx, tenantID, workflowID, "a1", "triggered", "a2")

	if err != nil {
		panic(err)
	}

	err = storage.AddDep(ctx, tenantID, workflowID, "a2", "success", "a3")

	if err != nil {
		panic(err)
	}

	go workflow.Run(ctx)

	nctx, ncancel := signal.NotifyContext(ctx, os.Interrupt)
	defer ncancel()

	<-nctx.Done()

	cancel()
	_ = workflow.Close(ctx)
}
