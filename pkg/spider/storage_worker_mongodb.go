package spider

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func InitMongodDBWorkerStorageAdapter(ctx context.Context) (*MongodDBWorkerStorageAdapter, error) {

	type Env struct {
		MongoDBURI  string `env:"MONGODB_URI,required"`
		MongoDBName string `env:"MONGODB_DB_NAME,required"`
	}

	var env Env

	err := envconfig.Process(ctx, &env)

	if err != nil {
		return nil, err
	}

	client, err := mongo.Connect(options.Client().ApplyURI(env.MongoDBURI))

	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, readpref.Primary())

	if err != nil {
		return nil, err
	}

	db := client.Database(env.MongoDBName)

	a := NewMongodDBWorkerStorageAdapter(client, db)

	return a, nil
}

type MongodDBWorkerStorageAdapter struct {
	client                             *mongo.Client
	workflowActionCollection           *mongo.Collection
	workflowCollection                 *mongo.Collection
	workflowExecutionHistoryCollection *mongo.Collection
}

func NewMongodDBWorkerStorageAdapter(client *mongo.Client, db *mongo.Database) *MongodDBWorkerStorageAdapter {
	return &MongodDBWorkerStorageAdapter{
		client:                             client,
		workflowActionCollection:           db.Collection("workflow_actions"),
		workflowCollection:                 db.Collection("workflows"),
		workflowExecutionHistoryCollection: db.Collection("workflow_execution_history"),
	}
}

// WorkflowExecutionHistory represents a workflow execution history record
type WorkflowExecutionHistory struct {
	TenantID   string    `bson:"tenant_id"`
	WorkflowID string    `bson:"workflow_id"`
	ActionKey  string    `bson:"action_key"`
	ActionID   string    `bson:"action_id"`
	UserID     string    `bson:"user_id"`
	Status     string    `bson:"status"` // success, failed
	RetryCount int       `bson:"retry_count"`
	ExecutedAt time.Time `bson:"executed_at"`
}

// GetExecutionHistory retrieves a history record by unique key
func (w *MongodDBWorkerStorageAdapter) GetExecutionHistory(ctx context.Context, tenantID, workflowID, actionKey, userID string) (*WorkflowExecutionHistory, error) {
	filter := bson.M{
		"tenant_id":   tenantID,
		"workflow_id": workflowID,
		"action_key":  actionKey,
		"user_id":     userID,
	}

	var history WorkflowExecutionHistory
	err := w.workflowExecutionHistoryCollection.FindOne(ctx, filter).Decode(&history)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// UpsertExecutionHistory inserts or updates a history record
func (w *MongodDBWorkerStorageAdapter) UpsertExecutionHistory(ctx context.Context, record *WorkflowExecutionHistory) error {
	filter := bson.M{
		"tenant_id":   record.TenantID,
		"workflow_id": record.WorkflowID,
		"action_key":  record.ActionKey,
		"user_id":     record.UserID,
	}

	update := bson.M{
		"$set": bson.M{
			"action_id":   record.ActionID,
			"status":      record.Status,
			"retry_count": record.RetryCount,
			"executed_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"tenant_id":   record.TenantID,
			"workflow_id": record.WorkflowID,
			"action_key":  record.ActionKey,
			"user_id":     record.UserID,
		},
	}

	opts := options.UpdateOne().SetUpsert(true)
	_, err := w.workflowExecutionHistoryCollection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (w *MongodDBWorkerStorageAdapter) GetAllConfigs(ctx context.Context, actionID string) ([]WorkerConfig, error) {

	// Use aggregation pipeline to join with workflows collection and filter by status
	pipeline := bson.A{
		// Match workflow_actions by action_id
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "action_id", Value: actionID},
			}},
		},
		// Lookup (join) with workflows collection
		bson.D{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "workflows"},
				{Key: "localField", Value: "workflow_id"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "workflow"},
			}},
		},
		// Unwind the workflow array (convert from array to object)
		bson.D{
			{Key: "$unwind", Value: "$workflow"},
		},
		// Filter to only include actions from active workflows
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "workflow.status", Value: "active"},
			}},
		},
	}

	cur, err := w.workflowActionCollection.Aggregate(ctx, pipeline)

	if err != nil {
		return nil, err
	}

	var confs []WorkerConfig

	for cur.TryNext(ctx) {

		// Temporary struct to hold aggregation result with workflow data
		var result struct {
			MDWorkflowAction `bson:",inline"`
			Workflow         struct {
				Name string `bson:"name"`
			} `bson:"workflow"`
		}

		err := cur.Decode(&result)

		if err != nil {
			return nil, err
		}

		confs = append(confs, WorkerConfig{
			WorkflowActionID: result.ID,
			TenantID:         result.TenantID,
			WorkflowID:       result.WorkflowID,
			WorkflowName:     result.Workflow.Name,
			Key:              result.Key,
			Config:           result.Config,
			Meta:             result.Meta,
		})
	}

	return confs, nil
}

func (w *MongodDBWorkerStorageAdapter) Close(ctx context.Context) error {
	return w.client.Disconnect(ctx)
}
