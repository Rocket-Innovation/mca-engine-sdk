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

// GetWorkflowEventsTopic returns topic name based on environment
// Topics: mca.workflow.events.local, mca.workflow.events.uat, mca.workflow.events.prod
func GetWorkflowEventsTopic() string {
	return fmt.Sprintf("mca.workflow.events.%s", getEnv())
}

// GetWorkflowRecipientsTopic returns topic name for workflow recipients
// Topics: mca.workflow.recipients.local, mca.workflow.recipients.uat, mca.workflow.recipients.prod
func GetWorkflowRecipientsTopic() string {
	return fmt.Sprintf("mca.workflow.recipients.%s", getEnv())
}

// GetWorkflowNotificationsTopic returns topic name for notification executions
// Topics: mca.workflow.notifications.local, mca.workflow.notifications.uat, mca.workflow.notifications.prod
func GetWorkflowNotificationsTopic() string {
	return fmt.Sprintf("mca.workflow.notifications.%s", getEnv())
}
