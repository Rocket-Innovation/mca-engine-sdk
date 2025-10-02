package apis

import (
	"github.com/targc/spider-go/pkg/spider"
	"github.com/targc/spider-go/pkg/spider/usecase"
	"github.com/gofiber/fiber/v2"
)

// CreateFlow godoc
// @Summary Create a new flow
// @Description Create a new workflow flow for a tenant
// @Tags flows
// @Accept json
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param payload body CreateFlowPayload true "Flow creation payload"
// @Success 200 {object} usecase.FlowResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tenants/{tenant_id}/flows [post]
func (h *Handler) CreateFlow(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "tenant_id is required",
		})
	}

	var payload struct {
		Name        string                    `json:"name"`
		TriggerType spider.FlowTriggerType   `json:"trigger_type"`
		Meta        map[string]string         `json:"meta,omitempty"`
		Actions     []WorkflowAction          `json:"actions"`
		Peers       []Peer                    `json:"peers"`
	}

	err := c.BodyParser(&payload)
	if err != nil {
		return err
	}

	if payload.Name == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "name is required",
		})
	}

	// Convert to usecase types
	actions := make([]usecase.WorkflowActionInput, len(payload.Actions))
	for i, action := range payload.Actions {
		actions[i] = usecase.WorkflowActionInput{
			Key:      action.Key,
			ActionID: action.ActionID,
			Config:   action.Config,
			Mapper:   action.Mapper,
			Meta:     action.Meta,
		}
	}

	peers := make([]usecase.PeerInput, len(payload.Peers))
	for i, peer := range payload.Peers {
		peers[i] = usecase.PeerInput{
			ParentKey:  peer.ParentKey,
			MetaOutput: peer.MetaOutput,
			ChildKey:   peer.ChildKey,
		}
	}

	req := &usecase.CreateFlowRequest{
		TenantID:    tenantID,
		Name:        payload.Name,
		TriggerType: payload.TriggerType,
		Meta:        payload.Meta,
		Actions:     actions,
		Peers:       peers,
	}

	result, err := h.usecase.CreateFlow(c.Context(), req)
	if err != nil {
		return c.Status(500).JSON(map[string]string{
			"error": "Failed to create flow",
		})
	}

	return c.JSON(result)
}

// ListFlows godoc
// @Summary List flows
// @Description Get a paginated list of flows for a tenant
// @Tags flows
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} spider.FlowListResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tenants/{tenant_id}/flows [get]
func (h *Handler) ListFlows(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "tenant_id is required",
		})
	}

	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}

	pageSize := c.QueryInt("page_size", 20)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result, err := h.usecase.ListFlows(c.Context(), tenantID, page, pageSize)
	if err != nil {
		return c.Status(500).JSON(map[string]string{
			"error": "Failed to list flows",
		})
	}

	return c.JSON(result)
}

// GetFlow godoc
// @Summary Get flow details
// @Description Get detailed information about a specific flow
// @Tags flows
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param id path string true "Flow ID"
// @Success 200 {object} usecase.FlowDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tenants/{tenant_id}/flows/{id} [get]
func (h *Handler) GetFlow(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "tenant_id is required",
		})
	}

	flowID := c.Params("id")
	if flowID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "flow id is required",
		})
	}

	result, err := h.usecase.GetFlow(c.Context(), tenantID, flowID)
	if err != nil {
		return c.Status(404).JSON(map[string]string{
			"error": "Flow not found",
		})
	}

	return c.JSON(result)
}

// UpdateFlow godoc
// @Summary Update a flow
// @Description Update an existing flow's properties
// @Tags flows
// @Accept json
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param flow_id path string true "Flow ID"
// @Param payload body UpdateFlowPayload true "Flow update payload"
// @Success 200 {object} spider.Flow
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tenants/{tenant_id}/flows/{flow_id} [put]
func (h *Handler) UpdateFlow(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "tenant_id is required",
		})
	}

	flowID := c.Params("flow_id")
	if flowID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "flow_id is required",
		})
	}

	var payload struct {
		Name        string                    `json:"name"`
		TriggerType spider.FlowTriggerType   `json:"trigger_type"`
		Meta        map[string]string         `json:"meta,omitempty"`
		Status      spider.FlowStatus         `json:"status"`
	}

	err := c.BodyParser(&payload)
	if err != nil {
		return err
	}

	if payload.Name == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "name is required",
		})
	}

	req := &usecase.UpdateFlowRequest{
		TenantID:    tenantID,
		FlowID:      flowID,
		Name:        payload.Name,
		TriggerType: payload.TriggerType,
		Meta:        payload.Meta,
		Status:      payload.Status,
	}

	flow, err := h.usecase.UpdateFlow(c.Context(), req)
	if err != nil {
		return c.Status(500).JSON(map[string]string{
			"error": "Failed to update flow",
		})
	}

	return c.JSON(flow)
}

// DeleteFlow godoc
// @Summary Delete a flow
// @Description Delete an existing flow
// @Tags flows
// @Param tenant_id path string true "Tenant ID"
// @Param flow_id path string true "Flow ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tenants/{tenant_id}/flows/{flow_id} [delete]
func (h *Handler) DeleteFlow(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "tenant_id is required",
		})
	}

	flowID := c.Params("flow_id")
	if flowID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "flow_id is required",
		})
	}

	err := h.usecase.DeleteFlow(c.Context(), tenantID, flowID)
	if err != nil {
		return c.Status(500).JSON(map[string]string{
			"error": "Failed to delete flow",
		})
	}

	return c.Status(204).Send(nil)
}