package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// StatsHistoryHandler handles GET /api/v1/system/stats/history.
func StatsHistoryHandler(svc *services.StatsHistoryService) fiber.Handler {
	return func(c fiber.Ctx) error {
		sinceStr := c.Query("since")
		if sinceStr != "" {
			since, err := strconv.ParseInt(sinceStr, 10, 64)
			if err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "invalid since parameter")
			}
			return c.JSON(svc.GetHistorySince(since))
		}
		return c.JSON(svc.GetHistory())
	}
}
