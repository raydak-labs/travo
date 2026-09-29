package api

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// ListServicesHandler handles GET /api/v1/services.
func ListServicesHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		list, err := sm.ListServices()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(list)
	}
}

// InstallServiceHandler handles POST /api/v1/services/:id/install.
func InstallServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Install(id); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// RemoveServiceHandler handles POST /api/v1/services/:id/remove.
func RemoveServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Remove(id); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// StartServiceHandler handles POST /api/v1/services/:id/start.
func StartServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Start(id); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// StopServiceHandler handles POST /api/v1/services/:id/stop.
func StopServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Stop(id); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// SetAutoStartHandler handles POST /api/v1/services/:id/autostart.
func SetAutoStartHandler(mgr *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := mgr.SetAutoStart(id, body.Enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
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

// streamLogBuffer is how many log lines may be pending for a slow client.
const streamLogBuffer = 256

// streamServiceAction sets streaming headers and runs action with real-time NDJSON output.
//
// The action is decoupled from the socket through a bounded queue, and a full
// queue drops lines rather than blocking. Progress output is best-effort: a
// browser tab that stops reading must not be able to stall the operation it is
// watching (a blocked write would hold up the command's output reader, which
// execx.Stream surfaces as a WaitDelay timeout — reporting a failed install
// that actually succeeded on the device).
func streamServiceAction(c fiber.Ctx, action func(logFn func(string)) error) error {
	c.Set("Content-Type", "application/x-ndjson")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Content-Type-Options", "nosniff")
	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		lines := make(chan string, streamLogBuffer)
		drained := make(chan struct{})
		go func() {
			defer close(drained)
			for line := range lines {
				writeStreamEvent(w, streamLogEvent{Type: "log", Data: line})
			}
		}()

		logFn := func(line string) {
			select {
			case lines <- line:
			default: // client is not keeping up; drop rather than block
			}
		}
		err := action(logFn)
		close(lines)
		<-drained // the writer must finish before the response body is closed

		if err != nil {
			writeStreamEvent(w, streamLogEvent{Type: "error", Data: err.Error()})
		} else {
			writeStreamEvent(w, streamLogEvent{Type: "done"})
		}
	})
	return nil
}

// InstallServiceStreamHandler handles POST /api/v1/services/:id/install/stream.
// Returns NDJSON with real-time install output.
func InstallServiceStreamHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		return streamServiceAction(c, func(logFn func(string)) error {
			return sm.InstallWithLog(id, logFn)
		})
	}
}

// RemoveServiceStreamHandler handles POST /api/v1/services/:id/remove/stream.
// Returns NDJSON with real-time remove output.
func RemoveServiceStreamHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		return streamServiceAction(c, func(logFn func(string)) error {
			return sm.RemoveWithLog(id, logFn)
		})
	}
}
