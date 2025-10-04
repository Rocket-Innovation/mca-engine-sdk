package apis

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

// DisableAction godoc
// @Summary Disable a workflow action
// @Description Disable a specific workflow action
// @Tags actions
// @Param tenant_id path string true "Tenant ID"
// @Param workflow_id path string true "Workflow ID"
// @Param key path string true "Action Key"
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tenants/{tenant_id}/workflows/{workflow_id}/actions/{key}/disable [post]
func (h *Handler) DisableAction(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "tenant_id is required",
		})
	}

	workflowID := c.Params("workflow_id")
	if workflowID == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "workflow_id is required",
		})
	}

	key := c.Params("key")
	if key == "" {
		return c.Status(400).JSON(map[string]string{
			"error": "key is required",
		})
	}

	err := h.usecase.DisableAction(c.Context(), tenantID, workflowID, key)
	if err != nil {
		return err
	}

	slog.Info("[process] disabled")

	return nil
}
