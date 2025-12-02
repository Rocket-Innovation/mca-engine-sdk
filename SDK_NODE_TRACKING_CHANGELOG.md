# SDK Node Tracking Implementation - Changelog

**Date:** 2025-12-02
**Status:** ✅ SDK Updated (v1.16.0)
**Related:** [mca-document/meeting/IMPLEMENTATION_ROADMAP_DEC_2025.md](../mca-document/meeting/IMPLEMENTATION_ROADMAP_DEC_2025.md)

---

## Summary

Updated mca-engine-sdk to support `node_id` and `node_name` fields in workflow event publishing. These fields enable better node-level tracking in the reporting system.

**Changes:**
- ✅ Added `NodeID` and `NodeName` fields to `WorkflowEventPayload`
- ✅ Updated `PublishEntered()` to include node tracking
- ✅ Updated `PublishExited()` to include node tracking
- ✅ Updated convenience methods `PublishExitedSuccess()` and `PublishExitedFailed()`
- ✅ Updated internal workflow engine (`pkg/spider/workflow.go`) to use new API
- ✅ Added `extractNodeName()` helper function

---

## Files Modified

### 1. Event Types
**File:** `pkg/events/types.go` ✅ **UPDATED**

**Lines 70-71** - Added new fields:

```go
type WorkflowEventPayload struct {
    // ... existing fields ...
    ExecutionStatus string `json:"execution_status,omitempty"` // For node_executions table
    NodeID          string `json:"node_id,omitempty"`          // NEW - For workflow_events table
    NodeName        string `json:"node_name,omitempty"`        // NEW - For workflow_events table
    ActionKey       string `json:"action_key,omitempty"`
    ActionID        string `json:"action_id,omitempty"`
    ActionLabel     string `json:"action_label,omitempty"`     // For node_executions table (backward compatibility)
    // ... rest of fields ...
}
```

---

### 2. Publisher Methods
**File:** `pkg/events/publisher.go` ✅ **UPDATED**

#### 2.1 PublishEntered() - Lines 170-197

**Before:**
```go
func (p *Publisher) PublishEntered(
    ctx context.Context,
    tenantID, workflowID, workflowName, sessionID, taskID string,
    recipientID string, recipientType RecipientType,
    actionKey, actionID string, // ❌ Missing node info
) error
```

**After:**
```go
func (p *Publisher) PublishEntered(
    ctx context.Context,
    tenantID, workflowID, workflowName, sessionID, taskID string,
    recipientID string, recipientType RecipientType,
    nodeID, nodeName string, // ✅ NEW - from workflow definition
    actionKey, actionID, actionLabel string,
) error {
    event := &WorkflowEventPayload{
        // ... existing fields ...
        NodeID:      nodeID,      // NEW
        NodeName:    nodeName,    // NEW
        ActionKey:   actionKey,
        ActionID:    actionID,
        ActionLabel: actionLabel, // For backward compatibility
        // ...
    }
}
```

---

#### 2.2 PublishExited() - Lines 199-233

**Before:**
```go
func (p *Publisher) PublishExited(
    ctx context.Context,
    tenantID, workflowID, workflowName, sessionID, taskID string,
    recipientID string, recipientType RecipientType,
    actionKey, actionID string, // ❌ Missing node info
    status string,
    errorMessage string,
    payload map[string]interface{},
) error
```

**After:**
```go
func (p *Publisher) PublishExited(
    ctx context.Context,
    tenantID, workflowID, workflowName, sessionID, taskID string,
    recipientID string, recipientType RecipientType,
    nodeID, nodeName string, // ✅ NEW - from workflow definition
    actionKey, actionID, actionLabel string,
    status string,
    errorMessage string,
    payload map[string]interface{},
) error {
    event := &WorkflowEventPayload{
        // ... existing fields ...
        NodeID:      nodeID,      // NEW (for workflow_events table)
        NodeName:    nodeName,    // NEW (for workflow_events table)
        ActionKey:   actionKey,
        ActionID:    actionID,
        ActionLabel: actionLabel, // For backward compatibility
        // ...
    }
}
```

---

#### 2.3 PublishExitedSuccess() - Lines 235-246

**Updated signature** to pass new parameters:
```go
func (p *Publisher) PublishExitedSuccess(
    ctx context.Context,
    tenantID, workflowID, workflowName, sessionID, taskID string,
    recipientID string, recipientType RecipientType,
    nodeID, nodeName string, // NEW
    actionKey, actionID, actionLabel string,
    payload map[string]interface{},
) error
```

---

#### 2.4 PublishExitedFailed() - Lines 248-259

**Updated signature** to pass new parameters:
```go
func (p *Publisher) PublishExitedFailed(
    ctx context.Context,
    tenantID, workflowID, workflowName, sessionID, taskID string,
    recipientID string, recipientType RecipientType,
    nodeID, nodeName string, // NEW
    actionKey, actionID, actionLabel string,
    errorMessage string,
) error
```

---

## Field Usage Clarification

| Field | Destination | Purpose |
|-------|-------------|---------|
| `node_id` | `workflow_events` table | Node ID from workflow definition |
| `node_name` | `workflow_events` table | Node display name |
| `action_key` | Both tables | Node key (a1, a2, etc.) |
| `action_id` | Both tables | Action type (line-action, query-action) |
| `action_label` | `node_executions` table | Node display name (backward compatibility) |
| `execution_status` | `node_executions` table | Detailed execution status |

**Why both `action_label` AND `node_name`?**
- `action_label` → Used by `node_executions` table (stateful, with UPDATE operations)
- `node_name` → Used by `workflow_events` table (append-only event log)

This separation allows us to phase out `action_label` from workflow_events while maintaining backward compatibility with stateful tables.

---

### 3. Workflow Engine (Internal)
**File:** `pkg/spider/workflow.go` ✅ **UPDATED**

#### 3.1 EventPublisher Interface - Lines 16-23

**Updated interface** to match new publisher signatures:

```go
type EventPublisher interface {
    PublishWorkflowStarted(...) error
    PublishWorkflowCompleted(...) error
    PublishEntered(ctx context.Context, tenantID, workflowID, workflowName, sessionID, taskID string,
        recipientID string, recipientType events.RecipientType,
        nodeID, nodeName string, // NEW
        actionKey, actionID, actionLabel string) error
    PublishExitedSuccess(ctx context.Context, tenantID, workflowID, workflowName, sessionID, taskID string,
        recipientID string, recipientType events.RecipientType,
        nodeID, nodeName string, // NEW
        actionKey, actionID, actionLabel string, payload map[string]interface{}) error
    PublishExitedFailed(...) error
    Close() error
}
```

#### 3.2 Node Name Extraction Helper - Lines 648-667

**New helper function** to extract node display name:

```go
// extractNodeName extracts node display name from WorkflowAction
// Priority: Meta["name"] > Config["label"] > Key (fallback)
func extractNodeName(action *WorkflowAction) string {
    // Try Meta["name"] first
    if action.Meta != nil {
        if name, ok := action.Meta["name"]; ok && name != "" {
            return name
        }
    }

    // Try Config["label"] second
    if action.Config != nil {
        if label, ok := action.Config["label"].(string); ok && label != "" {
            return label
        }
    }

    // Fallback to action key
    return action.Key
}
```

#### 3.3 Updated Publisher Calls

**Three locations updated** to pass node information:

**Location 1: listenTriggerMessages (lines 271-296)**
```go
// Publish entered event
if w.publisher != nil {
    // Extract node information from workflow action
    nodeID := dep.ID
    nodeName := extractNodeName(&dep)
    actionLabel := nodeName // Same as node name for backward compatibility

    err = w.publisher.PublishEntered(
        ctx, dep.TenantID, dep.WorkflowID, workflow.Name,
        sessionID, nextTaskID,
        recipientID, recipientType,
        nodeID, nodeName, // NEW
        dep.Key, dep.ActionID, actionLabel,
    )
}
```

**Location 2: listenOutputMessages - Exit Event (lines 381-407)**
```go
// Publish exited event (action completed successfully)
if w.publisher != nil {
    nodeID := workflowAction.ID
    nodeName := extractNodeName(workflowAction)
    actionLabel := nodeName

    err = w.publisher.PublishExitedSuccess(
        c.Context, m.TenantID, m.WorkflowID, workflow.Name,
        m.SessionID, m.TaskID,
        recipientID, recipientType,
        nodeID, nodeName, // NEW
        m.Key, workflowAction.ActionID, actionLabel,
        wvalues,
    )
}
```

**Location 3: listenOutputMessages - Next Node (lines 489-514)**
```go
// Publish entered event for the next action
if w.publisher != nil {
    nodeID := dep.ID
    nodeName := extractNodeName(&dep)
    actionLabel := nodeName

    err = w.publisher.PublishEntered(
        ctx, dep.TenantID, dep.WorkflowID, workflow.Name,
        m.SessionID, nextTaskID,
        recipientID, recipientType,
        nodeID, nodeName, // NEW
        dep.Key, dep.ActionID, actionLabel,
    )
}
```

---

## Breaking Changes ⚠️

**Status:** ✅ **RESOLVED - No External Breaking Changes**

### For Workflow Engine Users (mca-automation-workflow)
**NO CHANGES NEEDED** - The SDK's internal workflow engine (`pkg/spider/workflow.go`) has been updated to automatically extract and pass `node_id` and `node_name` from workflow definitions.

If you use `spider.InitWorkflowWithPublisher()`:
- ✅ Node tracking works automatically
- ✅ No code changes required
- ✅ Update to SDK v1.16.0 via `go get`

### For Direct Publisher Users (e.g., workers, custom services)
**Breaking Changes Apply** - If you directly instantiate `events.Publisher` and call publisher methods:

You must update calls to `PublishEntered()`, `PublishExited()`, `PublishExitedSuccess()`, or `PublishExitedFailed()` to provide:
- `nodeID` (from workflow action or worker input)
- `nodeName` (from workflow action or worker input)
- `actionLabel` (node display name for backward compatibility)

---

## Migration Guide for Callers

### Before:
```go
publisher.PublishEntered(
    ctx,
    tenantID, workflowID, workflowName, sessionID, taskID,
    recipientID, recipientType,
    actionKey, actionID, // ❌ Old signature
)
```

### After:
```go
// Get node info from workflow action
nodeID := action.ID  // or generate unique node ID
nodeName := getNodeName(action) // from action.Meta["name"] or action.Config["label"]
actionLabel := nodeName // backward compatibility

publisher.PublishEntered(
    ctx,
    tenantID, workflowID, workflowName, sessionID, taskID,
    recipientID, recipientType,
    nodeID, nodeName, // ✅ NEW
    actionKey, actionID, actionLabel, // ✅ NEW
)
```

---

## Where to Get node_id and node_name?

### Option 1: From WorkflowAction struct
```go
type WorkflowAction struct {
    ID         string            `json:"id"`   // Use as node_id
    Key        string            `json:"key"`  // action_key (a1, a2)
    ActionID   string            `json:"action_id"` // line-action, query-action
    Config     map[string]interface{} `json:"config"`
    Meta       map[string]string `json:"meta,omitempty"`
}

// Usage:
nodeID := action.ID
nodeName := action.Meta["name"] // or action.Config["label"]
if nodeName == "" {
    nodeName = action.Key // fallback to action key
}
```

### Option 2: From workflow definition (MongoDB)
If actions don't have node names, retrieve from workflow definition:
```go
workflow := getWorkflowFromMongoDB(workflowID)
node := workflow.Nodes[actionKey]
nodeID := node.ID
nodeName := node.Name
```

---

## Next Steps (Required)

### 1. Update Workflow Engine ✅ **COMPLETE**

**Repository:** `mca-engine-sdk`

The workflow engine has been updated to automatically extract `node_id` and `node_name` from workflow definitions:
- ✅ `EventPublisher` interface updated
- ✅ Three publisher calls updated
- ✅ `extractNodeName()` helper function added
- ✅ Extracts from `WorkflowAction.ID`, `Meta["name"]`, or `Config["label"]`

**mca-automation-workflow users:** Simply update to SDK v1.16.0, no code changes needed.

---

### 2. Update Workers (if they publish directly) ⏳

**Repository:** `mca-notification`

If workers call SDK publisher methods directly, update them:
```go
// LINE worker example
nodeID := input.NodeID
nodeName := input.NodeName
actionLabel := input.ActionLabel

publisher.PublishExited(
    ctx,
    input.TenantID, input.WorkflowID, input.WorkflowName,
    input.SessionID, input.TaskID,
    input.RecipientID, input.RecipientType,
    nodeID, nodeName, // NEW
    input.ActionKey, input.ActionID, actionLabel,
    "success", "", nil,
)
```

---

### 3. Update Worker Input Message ⏳

If workflow engine sends messages to workers via NATS, update message structure:

```go
type WorkerInput struct {
    TenantID      string
    WorkflowID    string
    WorkflowName  string
    SessionID     string
    TaskID        string
    RecipientID   string
    RecipientType string
    NodeID        string // NEW
    NodeName      string // NEW
    ActionKey     string
    ActionID      string
    ActionLabel   string // Keep for backward compatibility
    Config        map[string]any
}
```

---

## Testing

### 1. Verify SDK Compiles
```bash
cd mca-engine-sdk
go build ./pkg/events
# ✅ Should compile without errors
```

### 2. Update Callers and Test End-to-End
```bash
# After updating workflow engine and workers:
# 1. Start workflow
# 2. Check Kafka messages include node_id and node_name
# 3. Check workflow_events table has node_id and node_name populated
```

### 3. Verify Kafka Messages
```bash
# Subscribe to Kafka topic
kafka-console-consumer --topic mca.workflow.nodes.dev --from-beginning

# Should see messages like:
{
  "node_id": "node-001",
  "node_name": "Send Welcome LINE",
  "action_key": "a1",
  "action_id": "line-action",
  "action_label": "Send Welcome LINE",
  "event_type": "entered",
  ...
}
```

---

## Rollback Plan

If issues occur, revert SDK changes:

```bash
cd mca-engine-sdk
git log --oneline -5
# Find commit before SDK update
git revert <commit-hash>

# Rebuild and redeploy services using SDK
cd ../mca-automation-workflow
go mod tidy
go build
```

---

## Summary

✅ **SDK (mca-engine-sdk) - COMPLETE (v1.16.0)**
- Event types updated
- Publisher methods updated
- Internal workflow engine updated
- Node extraction helper function added
- Compiles successfully

✅ **Workflow Engine Integration - COMPLETE**
- SDK automatically extracts node_id from WorkflowAction.ID
- SDK automatically extracts node_name from Meta["name"] or Config["label"]
- No changes needed in mca-automation-workflow

⏳ **Workers - PENDING (Optional)**
- Workers need to receive node_id, node_name from workflow engine input
- Only if workers publish events directly (rare)

---

## Related Documentation

- [mca-document/report/WORKFLOW_EVENTS_BACKEND_IMPLEMENTATION.md](../../mca-document/report/WORKFLOW_EVENTS_BACKEND_IMPLEMENTATION.md)
- [mca-document/meeting/IMPLEMENTATION_ROADMAP_DEC_2025.md](../../mca-document/meeting/IMPLEMENTATION_ROADMAP_DEC_2025.md)
- [mca-automation-workflow/WORKFLOW_EVENTS_IMPLEMENTATION_CHANGELOG.md](../../mca-automation-workflow/WORKFLOW_EVENTS_IMPLEMENTATION_CHANGELOG.md)

---

**Last Updated:** December 2, 2025
**Version:** v1.16.0
**Status:** ✅ SDK and internal workflow engine complete - ready for production use
