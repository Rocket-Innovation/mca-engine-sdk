package spider

type WorkflowAction struct {
	ID         string                 `json:"id"`
	Key        string                 `json:"key"`
	TenantID   string                 `json:"tenant_id"`
	WorkflowID string                 `json:"workflow_id"`
	ActionID   string                 `json:"action_id"`
	NodeName   string                 `json:"node_name,omitempty"` // User-defined node name
	Config     map[string]interface{} `json:"config"`
	Map        map[string]Mapper      `json:"map"`
	Meta       map[string]string      `json:"meta,omitempty"`
	Disabled   bool                   `json:"disabled"`
}
