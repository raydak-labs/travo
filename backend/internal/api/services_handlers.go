package api

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// ListServicesHandler handles GET /api/v1/services.
func ListServicesHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		list, err := sm.ListServices()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(list)
	}
}

// InstallServiceHandler handles POST /api/v1/services/:id/install.
func InstallServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Install(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// RemoveServiceHandler handles POST /api/v1/services/:id/remove.
func RemoveServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Remove(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// StartServiceHandler handles POST /api/v1/services/:id/start.
func StartServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Start(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// StopServiceHandler handles POST /api/v1/services/:id/stop.
func StopServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Stop(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// SetAutoStartHandler handles POST /api/v1/services/:id/autostart.
func SetAutoStartHandler(mgr *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := mgr.SetAutoStart(id, body.Enabled); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// streamLogEvent represents a single NDJSON log event.
type streamLogEvent struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
}

// writeStreamEvent writes an NDJSON event line and flushes.
func writeStreamEvent(w *bufio.Writer, evt streamLogEvent) {
	data, _ := json.Marshal(evt)
	fmt.Fprintf(w, "%s\n", data)
	w.Flush()
}

// InstallServiceStreamHandler handles POST /api/v1/services/:id/install/stream.
// Returns NDJSON with real-time install output.
func InstallServiceStreamHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		c.Set("Content-Type", "application/x-ndjson")
		c.Set("Cache-Control", "no-cache")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			logFn := func(line string) {
				writeStreamEvent(w, streamLogEvent{Type: "log", Data: line})
			}
			if err := sm.InstallWithLog(id, logFn); err != nil {
				writeStreamEvent(w, streamLogEvent{Type: "error", Data: err.Error()})
			} else {
				writeStreamEvent(w, streamLogEvent{Type: "done"})
			}
		})
		return nil
	}
}

// RemoveServiceStreamHandler handles POST /api/v1/services/:id/remove/stream.
// Returns NDJSON with real-time remove output.
func RemoveServiceStreamHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		c.Set("Content-Type", "application/x-ndjson")
		c.Set("Cache-Control", "no-cache")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			logFn := func(line string) {
				writeStreamEvent(w, streamLogEvent{Type: "log", Data: line})
			}
			if err := sm.RemoveWithLog(id, logFn); err != nil {
				writeStreamEvent(w, streamLogEvent{Type: "error", Data: err.Error()})
			} else {
				writeStreamEvent(w, streamLogEvent{Type: "done"})
			}
		})
		return nil
	}
}
