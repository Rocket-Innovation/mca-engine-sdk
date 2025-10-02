# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Spider Go is a distributed workflow engine built in Go that orchestrates workers through NATS messaging and persists workflow state in MongoDB. The system enables composing complex, resilient workflows from individual workers that communicate asynchronously.

## Tech Stack

- **Language:** Go 1.24+
- **Messaging:** NATS JetStream (via `github.com/nats-io/nats.go`)
- **Database:** MongoDB (via `go.mongodb.org/mongo-driver/v2`)
- **Expression Engine:** `github.com/expr-lang/expr` for dynamic field mapping
- **Web Framework:** Fiber v2 for REST APIs
- **API Documentation:** Swagger/OpenAPI via `swaggo`

## Core Architecture

### Messaging Pattern

The system uses a message-driven architecture with three message types:

1. **TriggerMessage**: Initiates workflow execution (from triggers to workflow engine)
2. **InputMessage**: Sends work to workers (from workflow engine to workers)
3. **OutputMessage**: Returns results from workers (from workers to workflow engine)

All messages flow through NATS subjects and are processed by the workflow engine, which maintains execution state and orchestrates the flow.

### Key Components

**Workflow Engine** (`pkg/spider/workflow.go`)
- Listens for trigger and output messages
- Queries workflow definitions from storage
- Evaluates dependencies between actions
- Creates session contexts to track execution state
- Sends input messages to downstream workers
- Uses expression mapper to transform data between steps

**Workers** (`pkg/spider/worker.go`)
- Listen for input messages on action-specific NATS subjects
- Process work and optionally send output messages
- Can send trigger messages to initiate workflows
- Query their own configuration from MongoDB

**Adapters**
- `WorkflowMessengerAdapter` & `WorkerMessengerAdapter`: NATS communication
- `WorkflowStorageAdapter` & `WorkerStorageAdapter`: MongoDB persistence
- Default implementations auto-setup NATS streams and MongoDB schemas

### Session Management

Each workflow execution creates a session with:
- `SessionID`: Unique identifier for the workflow run
- `TaskID`: Unique identifier for each step in the workflow
- Session context: Map of previous step outputs available to expressions

Session contexts are stored in MongoDB and cleaned up after each step completes, allowing data to flow through the workflow DAG.

### Data Mapping

The `Mapper` system (`pkg/spider/storage.go`) supports three modes:
- `fixed`: Static values
- `key`: Direct field references
- `expression`: Dynamic expressions evaluated via `expr-lang/expr`

Expressions have access to:
- Previous step outputs via `$trigger` or step keys
- Built-in functions (e.g., `builtin.string()`)
- All session context data

### Multi-Tenancy

The system is multi-tenant with `TenantID` tracked throughout:
- Flows are scoped to tenants
- Workflow actions include tenant context
- Storage queries filter by tenant

## Development Commands

### Testing
```bash
go test ./...
```

### Generate Swagger Documentation
```bash
make swag
# OR
swag init -g cmd/workflow/main.go -o cmd/workflow/docs --parseDependency --parseInternal
```

### Run with Docker Compose
```bash
# Basic example with NATS, MongoDB, workflow engine, and sample workers
docker compose -f docker-compose.example-basic.yml up --build

# Set SLACK_WEBHOOK_URL environment variable for Slack notifications
export SLACK_WEBHOOK_URL=https://hooks.slack.com/...
docker compose -f docker-compose.example-basic.yml up --build
```

### Run Locally
```bash
# Set environment variables (see .env.example)
export NATS_HOST=localhost
export NATS_PORT=4222
export NATS_USER=root
export NATS_PASSWORD=root
export MONGODB_URI=mongodb://root:password@localhost:27017/
export MONGODB_DB_NAME=wf

# Run workflow engine
go run cmd/workflow/main.go

# Run individual workers
go run cmd/slack-worker/main.go
go run cmd/webhook-trigger/main.go
```

## Project Structure

```
cmd/                      # Production entry points
├── workflow/            # Main workflow engine with REST API
├── slack-worker/        # Slack notification worker
├── webhook-trigger/     # HTTP webhook trigger
├── cron-trigger/        # Scheduled trigger
├── control-flow-worker/ # Conditional logic worker
└── fd-order-worker/     # Custom business logic worker

pkg/spider/              # Core workflow engine library
├── workflow.go          # Workflow engine (listens trigger/output, sends input)
├── worker.go            # Worker runtime (listens input, sends output/trigger)
├── message.go           # Message type definitions
├── messenger.go         # Messenger adapter interfaces
├── storage.go           # Storage adapter interfaces
├── action.go            # Workflow action definitions
├── flow.go              # Flow/workflow metadata
├── messenger_*_nats.go  # NATS implementations
├── storage_*_mongodb.go # MongoDB implementations
├── apis/                # HTTP API handlers
└── usecase/             # Business logic layer

examples/                # Example applications
└── basic/              # Simple A->B workflow example

deploys/                # Deployment configurations
└── k8s/                # Kubernetes manifests
```

## Common Patterns

### Creating a New Worker

1. Define an action ID constant (e.g., `const actionID = "my-action"`)
2. Initialize worker: `spider.InitDefaultWorker(ctx, actionID)`
3. Implement handler function: `func(c spider.InputMessageContext, m spider.InputMessage) error`
4. Parse input values from JSON string `m.Values`
5. Send output via `c.SendOutput(metaOutput, values)` where values is JSON string
6. Register handler: `worker.Run(ctx, handler)`

### Creating a Trigger

1. Initialize worker with trigger action ID
2. Build TriggerMessage with workflow details
3. Call `worker.SendTriggerMessage(ctx, triggerMessage)`
4. Workflow engine will create session and start execution

### Adding API Endpoints

1. Add handler method to `pkg/spider/apis/apis.go`
2. Add use case method to `pkg/spider/usecase/`
3. Register route in `cmd/workflow/main.go`
4. Add Swagger annotations for documentation
5. Run `make swag` to regenerate docs

## Environment Configuration

Required environment variables (see [.env.example](.env.example)):
- `NATS_HOST`, `NATS_PORT`, `NATS_USER`, `NATS_PASSWORD`: NATS connection
- `NATS_STREAM_PREFIX`, `NATS_CONSUMER_ID_PREFIX`: NATS stream namespacing
- `MONGODB_URI`, `MONGODB_DB_NAME`: MongoDB connection
- `SLACK_WEBHOOK_URL`: Optional for Slack worker

## API Documentation

When the workflow engine is running, Swagger UI is available at:
- `http://localhost:8080/swagger/index.html`

Key API endpoints (all multi-tenant via `/tenants/:tenant_id`):
- `GET /tenants/:tenant_id/flows` - List flows
- `POST /tenants/:tenant_id/flows` - Create flow with actions and dependencies
- `PUT /tenants/:tenant_id/flows/:flow_id` - Update flow metadata
- `PUT /tenants/:tenant_id/workflows/:workflow_id/actions/:key` - Update action configuration
- `POST /tenants/:tenant_id/workflows/:workflow_id/actions/:key/disable` - Disable action

## Important Implementation Details

- **Expression Evaluation**: Expressions in mappers are evaluated at runtime using the `expr-lang/expr` library with session context as environment
- **Session Context Lifecycle**: Created before sending input, updated with output, deleted after processing
- **Concurrency**: Workflow engine processes dependencies concurrently (limit 10) using `errgroup`
- **Error Handling**: Errors are logged with `slog` but may halt workflow execution depending on context
- **NATS Subjects**: Dynamically created based on action IDs and message types
- **MongoDB Collections**: Separate collections for flows, actions, dependencies, and session contexts
