# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

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

### Configuration
- Environment variables for NATS and MongoDB connections
- Workers retrieve configuration through storage adapters
- Workflow definitions stored in MongoDB with action mappings using expr-lang syntax

### API Layer
REST API (`cmd/workflow/main.go`) provides workflow management endpoints with Swagger documentation at `/swagger/`. Key endpoints include tenant-scoped workflow CRUD operations and action management.
## Platform Documentation

### 📚 Centralized Documentation

**All MCA platform documentation is located in: `../mca-automation-workflow/MD/`**

This centralized folder contains comprehensive guides covering the entire MCA ecosystem:

#### Quick Access
- **[Documentation Index](../mca-automation-workflow/MD/README.md)** - Complete catalog (19+ files, 9,000+ lines)
- **[Architecture Guide](../mca-automation-workflow/MD/ARCHITECTURE_GUIDE.md)** - Complete system architecture
- **[Quick Start Guide](../mca-automation-workflow/MD/QUICK_START.md)** - 5-minute platform setup

#### Key Documentation Categories
- **Architecture & System** (2 files) - System design, workflow flows
- **Query & Conditions** (3 files) - Condition system, SQL generation, cross-event analysis
- **Frontend Features** (5 files) - UI components, variable helpers, node management
- **Custom Fields** (3 files) - Custom field implementation and integration
- **Testing & Integration** (2 files) - End-to-end testing, Playwright helpers
- **MCP Playwright** (3 files) - Browser automation integration

#### Related Repositories
- **[mca-front-end/CLAUDE.md](../mca-front-end/CLAUDE.md)** - Frontend workflow builder
- **[mca-automation-workflow/CLAUDE.md](../mca-automation-workflow/CLAUDE.md)** - Workflow execution engine
- **[mca-bigQuery/CLAUDE.md](../mca-bigQuery/CLAUDE.md)** - PostgreSQL query service

**Why Centralized in Backend Repository?**
- Backend is the primary working directory for development
- Single source of truth for platform architecture
- Consistent documentation across all services
- Easy access from any repository via relative paths
- Co-located with workflow engine (core of the platform)
