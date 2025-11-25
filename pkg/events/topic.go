package events

import (
	"fmt"
	"os"
)

// GetWorkflowEventsTopic returns topic name based on environment
// Topics: mca.workflow.events.local, mca.workflow.events.uat, mca.workflow.events.prod
func GetWorkflowEventsTopic() string {
	env := os.Getenv("APP_ENV") // local, uat, prod
	if env == "" {
		env = "local"
	}
	return fmt.Sprintf("mca.workflow.events.%s", env)
}
