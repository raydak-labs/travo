package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetDataUsageHandler returns current usage per interface from vnstat.
func GetDataUsageHandler(svc *services.DataUsageService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// ResetDataUsageHandler resets vnstat counters for a single interface.
func ResetDataUsageHandler(svc *services.DataUsageService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Interface string `json:"interface"`
		}
		if err := c.Bind().Body(&req); err != nil || req.Interface == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "interface is required")
		}
		if err := svc.ResetInterface(req.Interface); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetDataBudgetHandler returns the configured data budgets.
func GetDataBudgetHandler(svc *services.DataUsageService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetBudget()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(cfg)
	}
}

// SetDataBudgetHandler writes the data budget configuration.
func SetDataBudgetHandler(svc *services.DataUsageService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.DataBudgetConfig
		if err := c.Bind().Body(&cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if cfg.Budgets == nil {
			cfg.Budgets = []models.DataBudget{}
		}
		if err := svc.SetBudget(cfg); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
