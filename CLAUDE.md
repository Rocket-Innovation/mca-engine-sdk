# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project: MCA Engine SDK (Spider-Go)

**This is the shared workflow SDK library** - the runtime engine and worker framework used by all MCA services.

**Role:** Workflow execution engine, worker abstraction, NATS messaging, MongoDB storage adapters.

---

## 📚 Platform Documentation

**All cross-platform documentation is centralized in: `../mca-document/`**

For understanding the MCA platform as a whole, see:
- **[Documentation Index](../mca-document/README.md)** - Complete catalog of all platform docs
- **[Architecture Guide](../mca-document/ARCHITECTURE_GUIDE.md)** - Complete system architecture
- **[Quick Start Guide](../mca-document/QUICK_START.md)** - 5-minute platform setup

For service implementations:
- **[mca-automation-workflow/CLAUDE.md](../mca-automation-workflow/CLAUDE.md)** - Backend workflow service
- **[mca-front-end/CLAUDE.md](../mca-front-end/CLAUDE.md)** - Frontend workflow builder
- **[mca-bigQuery/CLAUDE.md](../mca-bigQuery/CLAUDE.md)** - PostgreSQL query service

---

## This Library Overview

**Spider-Go SDK** is a distributed workflow engine library built on NATS JetStream and MongoDB.

**What This Library Provides:**
- Workflow runtime engine
- Worker abstraction (spider.Worker interface)
- MongoDB storage adapters
- NATS messaging layer
- Configuration auto-loading
- Expression evaluation (expr-lang)

**Used By:**
- mca-automation-workflow (all 5 services)
- mca-bigQuery (worker mode)
- Any custom worker implementations

---

## Development Commands

### Building and Testing
```bash
# Run all tests
go test ./...

# Build all binaries
go build ./cmd/...

# Generate Swagger documentation
make swag
```

### Running Components
```bash
# Run basic example with Docker Compose
docker compose -f docker-compose.example-basic.yml up --build

# Run individual components (requires NATS and MongoDB running)
go run cmd/workflow/main.go        # Workflow engine
go run cmd/slack-worker/main.go    # Slack notification worker
go run cmd/webhook-trigger/main.go # HTTP webhook trigger
go run cmd/cron-trigger/main.go    # Cron-based trigger
```

---

## Architecture Overview

Spider Go is a distributed workflow engine built on a message-passing architecture using NATS JetStream for communication and MongoDB for persistence.

### Core Components

**Workflow Engine** (`pkg/spider/workflow.go`): Central orchestrator that:
- Listens for trigger messages to start new workflow sessions
- Listens for output messages from workers to continue workflow execution
- Manages workflow state and context through sessions
- Uses expression language (expr-lang) for dynamic data mapping between workflow steps

**Workers** (`pkg/spider/worker.go`): Execution units that:
- Listen for input messages on NATS subjects based on their action ID
- Process business logic and send output messages back to the workflow
- Can trigger new workflows via trigger messages
- Access configuration through the storage layer

**Storage Layer**: MongoDB-based persistence with two adapters:
- `WorkflowStorageAdapter`: Manages workflow definitions, actions, and session contexts
- `WorkerStorageAdapter`: Provides worker configuration access

**Messaging Layer**: NATS-based communication with adapters:
- `WorkflowMessengerAdapter`: Handles trigger/output message routing for workflows
- `WorkerMessengerAdapter`: Manages input/output messaging for individual workers

### Key Concepts

**Workflows**: Defined as DAGs of actions with dependencies managed through "peers" that specify parent-child relationships and conditional execution based on meta_output values.

**Sessions**: Runtime instances of workflows identified by session_id, containing context data that flows between workflow steps.

**Actions**: Reusable workflow steps identified by action_id, with configuration and input/output mapping defined per workflow.

**Triggers**: Entry points for workflows (webhook, cron, or event-based) that create initial session context.

### Message Flow
1. Triggers send `TriggerMessage` to start workflows
2. Workflow engine creates session context and sends `InputMessage` to workers
3. Workers process and send `OutputMessage` back to workflow engine
4. Engine updates session context and triggers dependent actions
5. Process continues until workflow completion

**Detailed flow diagrams:** [Architecture Guide](../mca-document/ARCHITECTURE_GUIDE.md)

### Configuration
- Environment variables for NATS and MongoDB connections
- Workers retrieve configuration through storage adapters
- Workflow definitions stored in MongoDB with action mappings using expr-lang syntax

### API Layer
REST API (`cmd/workflow/main.go`) provides workflow management endpoints with Swagger documentation at `/swagger/`. Key endpoints include tenant-scoped workflow CRUD operations and action management.

---

## Worker Pattern

Workers implement `spider.Worker`:
```go
type Worker interface {
    Run(ctx context.Context) error
    ActionID() string
    Process(ctx context.Context, input InputMessage) (OutputMessage, error)
}
```

**Key Features:**
- Listen to NATS for action_id-specific messages
- Process input and return output
- Acknowledgment-based delivery (at-least-once)

**Example Implementations:**
- `mca-automation-workflow/internal/app/slack_worker/` - Slack notifications
- `mca-automation-workflow/internal/app/control_flow_worker/` - Condition evaluation
- `mca-bigQuery/cmd/worker/` - PostgreSQL queries

---

## Key Files

| File | Purpose |
|------|---------|
| `pkg/spider/workflow.go` | Workflow engine core |
| `pkg/spider/worker.go` | Worker interface & base |
| `pkg/spider/action.go` | Action model |
| `pkg/storage/` | MongoDB adapters |
| `pkg/messenger/` | NATS adapters |
| `cmd/workflow/main.go` | API server example |

---

## Integration with MCA Services

This SDK is imported by all MCA services:

**mca-automation-workflow:**
```go
import "github.com/Rocket-Innovation/mca-engine-sdk/pkg/spider"
import "github.com/Rocket-Innovation/mca-engine-sdk/pkg/storage"
```

**mca-bigQuery:**
```go
import "github.com/Rocket-Innovation/mca-engine-sdk/pkg/spider"
```

**Version Management:**
- Services specify SDK version in go.mod
- Breaking changes require major version bump
- Feature additions use minor version increments

**Related Repositories:**
- **[mca-automation-workflow](../mca-automation-workflow/CLAUDE.md)** - Workflow execution engine
- **[mca-bigQuery](../mca-bigQuery/CLAUDE.md)** - PostgreSQL query service
- **[mca-notification](../mca-notification/CLAUDE.md)** - LINE notification worker
- **[mca-timer](../mca-timer/CLAUDE.md)** - Timer/callback platform (Rust, not using this SDK)
- **[mca-front-end](../mca-front-end/CLAUDE.md)** - React workflow builder UI
- **[roc-argocd](../roc-argocd/CLAUDE.md)** - Kubernetes deployment

---

## Recent Changes (This Library)

**v1.3.31+ (2025-11-21):**
- Execution history tracking support
- Workflow deduplication integration

**v1.3.17:**
- Multi-event trigger support
- Config: `map[string]interface{}` (supports array or string)

**v1.3.14:**
- Null expression support
- Enhanced error logging

---

## Documentation

**This Library:**
- Code examples in `cmd/` directory
- Swagger docs at `/swagger/` (when running API)

**Platform-Wide:**
- **[../mca-document/](../mca-document/)** - All cross-platform documentation