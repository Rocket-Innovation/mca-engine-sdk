package spider

import (
	"context"
	"log/slog"
)

type Worker struct {
	Messenger WorkerMessengerAdapter
	Storage   WorkerStorageAdapter
	ActionID  string
}

func InitDefaultWorker(
	ctx context.Context,
	actionID string,
) (*Worker, error) {
	messenger, err := InitNATSWorkerMessengerAdapter(ctx, actionID, InitNATSWorkerMessengerAdapterOpt{
		BetaAutoSetupNATS: true,
	})

	if err != nil {
		return nil, err
	}

	storage, err := InitMongodDBWorkerStorageAdapter(ctx)

	if err != nil {
		return nil, err
	}

	return &Worker{
		messenger,
		storage,
		actionID,
	}, nil
}

func (w *Worker) Run(ctx context.Context, h func(c InputMessageContext, m InputMessage) error) error {

	err := w.Messenger.ListenInputMessages(
		ctx,
		func(c InputMessageContext, m InputMessage) error {

			c.SendOutput = func(metaOutput string, values string) error {
				err := w.Messenger.SendOutputMessage(c.Context, m.ToOutputMessage(metaOutput, values))

				if err != nil {
					return err
				}

				return nil
			}

			err := h(c, m)

			if err != nil {
				slog.Error("failed to process handler", slog.String("error", err.Error()))
				return err
			}

			return nil
		},
	)

	return err
}

func (w *Worker) SendTriggerMessage(ctx context.Context, m TriggerMessage) error {

	m.ActionID = w.ActionID

	err := w.Messenger.SendTriggerMessage(ctx, m)

	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) GetAllConfigs(ctx context.Context) ([]WorkerConfig, error) {

	confs, err := w.Storage.GetAllConfigs(ctx, w.ActionID)

	if err != nil {
		return nil, err
	}

	return confs, nil
}

func (w *Worker) Close(ctx context.Context) error {

	err := w.Messenger.Close(ctx)

	if err != nil {
		return err
	}

	return nil
}

// GetExecutionHistory retrieves a workflow execution history record
func (w *Worker) GetExecutionHistory(ctx context.Context, tenantID, workflowID, actionKey, userID string) (*WorkflowExecutionHistory, error) {
	if mongoAdapter, ok := w.Storage.(*MongodDBWorkerStorageAdapter); ok {
		return mongoAdapter.GetExecutionHistory(ctx, tenantID, workflowID, actionKey, userID)
	}
	return nil, nil
}

// UpsertExecutionHistory inserts or updates a workflow execution history record
func (w *Worker) UpsertExecutionHistory(ctx context.Context, record *WorkflowExecutionHistory) error {
	if mongoAdapter, ok := w.Storage.(*MongodDBWorkerStorageAdapter); ok {
		return mongoAdapter.UpsertExecutionHistory(ctx, record)
	}
	return nil
}
