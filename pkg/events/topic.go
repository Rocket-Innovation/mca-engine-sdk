package events

import (
	"fmt"
	"os"
)

func getEnv() string {
	env := os.Getenv("ENVIRONMENT") // local, uat, prod
	if env == "" {
		env = "local"
	}
	return env
}

// GetWorkflowNodesTopic returns topic name for node executions (entered/exited)
// Topics: mca.workflow.nodes.local, mca.workflow.nodes.uat, mca.workflow.nodes.prod
func GetWorkflowNodesTopic() string {
	return fmt.Sprintf("mca.workflow.nodes.%s", getEnv())
}

// GetWorkflowExecutionsTopic returns topic name for workflow executions (started/completed/exited)
// Topics: mca.workflow.executions.local, mca.workflow.executions.uat, mca.workflow.executions.prod
func GetWorkflowExecutionsTopic() string {
	return fmt.Sprintf("mca.workflow.executions.%s", getEnv())
}

// GetWorkflowActionsTopic returns topic name for action executions (notifications, etc.)
// Topics: mca.workflow.actions.local, mca.workflow.actions.uat, mca.workflow.actions.prod
func GetWorkflowActionsTopic() string {
	return fmt.Sprintf("mca.workflow.actions.%s", getEnv())
}

// Deprecated: Use GetWorkflowNodesTopic instead
func GetWorkflowEventsTopic() string {
	return GetWorkflowNodesTopic()
}

// Deprecated: Use GetWorkflowExecutionsTopic instead
func GetWorkflowRecipientsTopic() string {
	return GetWorkflowExecutionsTopic()
}

// Deprecated: Use GetWorkflowActionsTopic instead
func GetWorkflowNotificationsTopic() string {
	return GetWorkflowActionsTopic()
}
