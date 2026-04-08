package api

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/services"
	"time"
)

// CreateSnapshotRequest represents a request to create a configuration snapshot.
type CreateSnapshotRequest struct {
	Description string   `json:"description"`
	ConfigNames []string `json:"config_names"`
}

// SnapshotListResponse represents the response for listing snapshots.
type SnapshotListResponse struct {
	Snapshots []*services.ConfigSnapshot `json:"snapshots"`
}

// CreateSnapshotHandler creates a new configuration snapshot.
func CreateSnapshotHandler(snapshotSvc *services.SnapshotService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req CreateSnapshotRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if req.Description == "" {
			req.Description = "Manual snapshot"
		}
		if len(req.ConfigNames) == 0 {
			req.ConfigNames = []string{"system", "network", "wireless", "firewall", "dhcp"}
		}

		snapshot, err := snapshotSvc.CreateSnapshot(req.Description, req.ConfigNames, "user")
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(http.StatusCreated).JSON(snapshot)
	}
}

// ListSnapshotsHandler returns all available snapshots.
func ListSnapshotsHandler(snapshotSvc *services.SnapshotService) fiber.Handler {
	return func(c fiber.Ctx) error {
		snapshots, err := snapshotSvc.ListSnapshots()
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(SnapshotListResponse{Snapshots: snapshots})
	}
}

// GetSnapshotHandler returns a specific snapshot by ID.
func GetSnapshotHandler(snapshotSvc *services.SnapshotService) fiber.Handler {
	return func(c fiber.Ctx) error {
		snapshotID := c.Params("id")
		if snapshotID == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Snapshot ID is required",
			})
		}

		snapshot, err := snapshotSvc.GetSnapshot(snapshotID)
		if err != nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"error": "Snapshot not found",
			})
		}

		return c.JSON(snapshot)
	}
}

// RestoreSnapshotHandler restores a configuration snapshot.
func RestoreSnapshotHandler(snapshotSvc *services.SnapshotService, applier services.UCIApplyConfirm) fiber.Handler {
	return func(c fiber.Ctx) error {
		snapshotID := c.Params("id")
		if snapshotID == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Snapshot ID is required",
			})
		}

		// Create auto-snapshot before restore
		if _, err := snapshotSvc.AutoSaveBeforeChange("before restore", []string{"system", "network", "wireless", "firewall", "dhcp"}, "system"); err != nil {
			// Log but continue
		}

		if err := snapshotSvc.RestoreSnapshot(snapshotID); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Apply the restored configuration
		if applier != nil {
			if err := applier.ApplyAndConfirm([]string{"system", "network", "wireless", "firewall", "dhcp"}); err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to apply restored configuration: " + err.Error(),
				})
			}
		}

		return c.JSON(fiber.Map{
			"message": "Configuration restored successfully",
			"snapshot_id": snapshotID,
		})
	}
}

// DeleteSnapshotHandler deletes a snapshot.
func DeleteSnapshotHandler(snapshotSvc *services.SnapshotService) fiber.Handler {
	return func(c fiber.Ctx) error {
		snapshotID := c.Params("id")
		if snapshotID == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Snapshot ID is required",
			})
		}

		if err := snapshotSvc.DeleteSnapshot(snapshotID); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Snapshot deleted successfully",
		})
	}
}

// CleanSnapshotsHandler cleans up old snapshots.
func CleanSnapshotsHandler(snapshotSvc *services.SnapshotService) fiber.Handler {
	return func(c fiber.Ctx) error {
		olderThanDays := c.Query("older_than_days", "30")
		days, err := strconv.Atoi(olderThanDays)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid older_than_days parameter",
			})
		}

		minSnapshots := c.Query("min_snapshots", "5")
		min, err := strconv.Atoi(minSnapshots)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid min_snapshots parameter",
			})
		}

		if err := snapshotSvc.CleanOldSnapshots(time.Duration(days)*24*time.Hour, min); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Snapshots cleaned up successfully",
		})
	}
}