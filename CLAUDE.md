# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Spider Go is a distributed workflow engine that orchestrates workers using NATS JetStream for messaging and MongoDB for state persistence. The system enables building resilient workflows where tasks survive restarts and failures through session-based state management.

## Core Architecture

### Message-Driven Design

The system uses three primary message types flowing through NATS:

- **TriggerMessage**: Initiates workflow execution (from external triggers or workers)
- **InputMessage**: Carries task input to workers with session context
- **OutputMessage**: Returns results from workers to the workflow engine

### Session-Based State Management

Each workflow execution creates a session with unique `sessionID`. As messages flow through the workflow:
- Context is stored in MongoDB before sending to workers (`CreateSessionContext`)
- Workers receive input, process it, and send output
- Context is retrieved (`GetSessionContext`), merged with output, deleted, then forwarded to next steps
- This ensures workflow state survives worker failures or restarts

### Adapter Pattern

The system uses adapters to decouple core logic from infrastructure:
- **WorkflowMessengerAdapter** / **WorkerMessengerAdapter**: NATS messaging (see [messenger_workflow_nats.go](pkg/spider/messenger_workflow_nats.go), [messenger_worker_nats.go](pkg/spider/messenger_worker_nats.go))
- **WorkflowStorageAdapter** / **WorkerStorageAdapter**: MongoDB persistence (see [storage_workflow_mongodb.go](pkg/spider/storage_workflow_mongodb.go), [storage_worker_mongodb.go](pkg/spider/storage_worker_mongodb.go))

### Expression-Based Mapping

Data transformation between workflow steps uses the `expr-lang/expr` library. Each WorkflowAction contains a `Map` field (map of Mappers) that defines how to transform context into worker input:

- **MapperModeFixed**: Static value
- **MapperModeKey**: Reference a key from context
- **MapperModeExpression**: Evaluate expression with context as environment

The `ex()` function in [workflow.go:347](pkg/spider/workflow.go#L347) compiles and executes these expressions. The environment includes all previous step outputs plus a `builtin` object with helper functions.

### Two-Process Model

1. **Workflow Engine** ([cmd/workflow](cmd/workflow)): Listens for trigger/output messages, queries MongoDB for workflow definitions and dependencies, transforms data via expressions, sends input messages to workers
2. **Workers** ([cmd/\*-worker](cmd)): Listen for input messages on action-specific subjects, process tasks, send output messages back to workflow

## Development Commands

### Testing
```bash
go test ./...
```

### Generate Swagger Docs
```bash
make swag
# Or manually:
swag init -g cmd/workflow/main.go -o cmd/workflow/docs --parseDependency --parseInternal
```

### Run Basic Example
```bash
docker compose -f docker-compose.example-basic.yml up --build
```

### Run Local Development Stack
```bash
# Ensure .env file exists (copy from .env.example)
docker compose -f docker-compose.local.yml up --build
```

## Environment Configuration

Required environment variables (see [.env.example](.env.example)):

- **NATS_HOST**, **NATS_PORT**, **NATS_USER**, **NATS_PASSWORD**: NATS connection
- **NATS_STREAM_PREFIX**: Prefix for NATS JetStream streams
- **NATS_CONSUMER_ID_PREFIX**: Prefix for NATS consumer IDs
- **MONGODB_URI**, **MONGODB_DB_NAME**: MongoDB connection
- **SLACK_WEBHOOK_URL**: Optional Slack integration

Workers read config using `github.com/sethvargo/go-envconfig`.

## Key Components

### Workflow ([pkg/spider/workflow.go](pkg/spider/workflow.go))

- `listenTriggerMessages()`: Handles workflow initiation, creates initial session context
- `listenOutputMessages()`: Handles worker outputs, updates context, forwards to dependencies
- Both methods query `QueryWorkflowActionDependencies()` to find next steps based on `metaOutput` routing

### Worker ([pkg/spider/worker.go](pkg/spider/worker.go))

- `Run()`: Listens for input messages, executes handler, provides `SendOutput()` function
- `SendTriggerMessage()`: Allows workers to trigger workflows
- `GetAllConfigs()`: Retrieves all workflow configs that use this worker's actionID

### Flow & WorkflowAction ([pkg/spider/flow.go](pkg/spider/flow.go), [action.go](pkg/spider/action.go))

- **Flow**: Workflow definition with trigger type (event/schedule) and status (draft/active)
- **WorkflowAction**: Node in workflow graph with actionID (worker type), config, mapping rules, and disabled flag

### Bootstrap Helper ([pkg/spider/bootstrap_worker.go](pkg/spider/bootstrap_worker.go))

`LazyBootstrapWorker()` provides simplified worker initialization with automatic signal handling - useful for simple workers (see [examples/basic/cmd/worker-b](examples/basic/cmd/worker-b)).

## API Structure

The workflow engine exposes REST APIs ([cmd/workflow/main.go](cmd/workflow/main.go)):

- Flow management: `GET/POST/PUT/DELETE /tenants/:tenant_id/flows/...`
- Action management: `PUT /tenants/:tenant_id/workflows/:workflow_id/actions/:key`, `POST .../actions/:key/disable`
- Swagger UI: `/swagger/*`

All operations are multi-tenant with `tenant_id` scope.

## Working with Examples

The [examples/basic](examples/basic) demonstrates the full flow:
- **worker-a**: HTTP trigger that sends TriggerMessage
- **workflow**: Orchestrates the flow
- **worker-b**: Receives input, posts to Slack, sends output

Study this example to understand message flow and session handling.

## Important Notes

- Always use `uuid.NewV7()` for generating session/task IDs (time-ordered UUIDs)
- Error handling logs to slog but returns errors for NATS to handle retries
- Dependencies support `metaOutput` routing - same action can have different next steps based on output type
- The `$trigger` special key in context always references the triggering action's output
- Workflow version management exists (`Flow.Version`) but is not yet fully implemented
