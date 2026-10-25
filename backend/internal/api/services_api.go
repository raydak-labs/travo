package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/system"
)

type serviceIDParam struct {
	ID string `path:"id"`
}

type autostartInput struct {
	ID   string `path:"id"`
	Body EnabledRequest
}

// StreamEvent is one NDJSON line of an install or remove stream: "log" lines,
// then a final "done" or "error".
type StreamEvent struct {
	Type string `json:"type" enum:"log,done,error"`
	Data string `json:"data,omitempty"`
}

// streamLogBuffer is how many log lines may be pending for a slow client.
const streamLogBuffer = 256

func registerServices(api *API, sm *system.ServiceManager) {
	get(api, "/api/v1/services", "ListServices", "Optional packages and their state",
		func(context.Context, *none) (*Body[[]models.ServiceInfo], error) {
			return replyOrFail(sm.ListServices())
		})

	// Errors pass through unprefixed: they name the failing package, and the
	// requests carry no body that could have been malformed. An unknown id is a
	// genuine 400; a failed install answers the same until the service manager
	// can tell the two apart.
	for _, a := range []struct {
		action, id, summary string
		fn                  func(string) error
	}{
		{"install", "InstallService", "Install a package", sm.Install},
		{"remove", "RemoveService", "Remove a package", sm.Remove},
		{"start", "StartService", "Start a service", sm.Start},
		{"stop", "StopService", "Stop a service", sm.Stop},
	} {
		post(api, "/api/v1/services/{id}/"+a.action, a.id, a.summary,
			func(_ context.Context, in *serviceIDParam) (*Body[Status], error) {
				if err := a.fn(in.ID); err != nil {
					return nil, badRequest(err.Error())
				}
				return statusOK, nil
			})
	}
	post(api, "/api/v1/services/{id}/autostart", "SetServiceAutoStart", "Enable or disable start at boot",
		func(_ context.Context, in *autostartInput) (*Body[Status], error) {
			return okOrFail(sm.SetAutoStart(in.ID, in.Body.Enabled))
		})

	for _, a := range []struct {
		action, id, summary string
		fn                  func(string, func(string)) error
	}{
		{"install", "InstallServiceStream", "Install a package, streaming its output", sm.InstallWithLog},
		{"remove", "RemoveServiceStream", "Remove a package, streaming its output", sm.RemoveWithLog},
	} {
		post(api, "/api/v1/services/{id}/"+a.action+"/stream", a.id, a.summary,
			func(ctx context.Context, in *serviceIDParam) (*Stream, error) {
				return streamAction(ctx, func(logFn func(string)) error { return a.fn(in.ID, logFn) }), nil
			}, ndjson)
	}
}

// ndjson documents a streamed NDJSON response.
func ndjson(op *huma.Operation) {
	op.Responses = map[string]*huma.Response{
		"200": {
			Description: "application/x-ndjson stream of StreamEvent lines",
			Content:     map[string]*huma.MediaType{"application/x-ndjson": {}},
		},
	}
}

// streamAction runs action and streams its log as NDJSON.
//
// The action is decoupled from the socket through a bounded queue, and a full
// queue drops lines rather than blocking: a browser tab that stops reading
// must not stall the install it is watching (execx.Stream would surface the
// blocked reader as a WaitDelay timeout and report a failed install that
// succeeded on the device).
func streamAction(ctx context.Context, action func(logFn func(string)) error) *Stream {
	return &Stream{Write: func() {
		c := fiberCtx(ctx)
		c.Status(http.StatusOK)
		c.Set("Content-Type", "application/x-ndjson")
		c.Set("Cache-Control", "no-cache")
		c.Set("X-Content-Type-Options", "nosniff")
		c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
			lines := make(chan string, streamLogBuffer)
			drained := make(chan struct{})
			go func() {
				defer close(drained)
				for line := range lines {
					writeStreamEvent(w, StreamEvent{Type: "log", Data: line})
				}
			}()
			err := action(func(line string) {
				select {
				case lines <- line:
				default: // client is not keeping up; drop rather than block
				}
			})
			close(lines)
			<-drained // the writer must finish before the body closes
			if err != nil {
				writeStreamEvent(w, StreamEvent{Type: "error", Data: err.Error()})
			} else {
				writeStreamEvent(w, StreamEvent{Type: "done"})
			}
		})
	}}
}

// writeStreamEvent writes one NDJSON line and flushes. A write error means
// the client went away; the stream ends with it.
func writeStreamEvent(w *bufio.Writer, evt StreamEvent) {
	data, _ := json.Marshal(evt)
	_, _ = w.Write(append(data, '\n'))
	_ = w.Flush()
}
