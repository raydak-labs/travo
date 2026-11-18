package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/blackdark-org/travo-greenfield/internal/throughput"
)

const (
	dashboardStreamLimit    = 8
	dashboardHeartbeat      = 15 * time.Second
	dashboardWriteTimeout   = 5 * time.Second
	dashboardStatusInterval = 5 * time.Second
	// The Tailscale read runs the tailscale CLI twice; tailnet state changes slowly.
	dashboardTailscaleInterval = 15 * time.Second
	dashboardAlertsInterval    = 15 * time.Second
	dashboardInitialComment    = ": connected\n\n"
	dashboardHeartbeatComment  = ": heartbeat\n\n"
)

type sourceUnavailableBody struct {
	Source string `json:"source" enum:"connectivity,resources,throughput,alerts,tailscale"`
}

func (s *server) registerDashboardEvents(a huma.API) {
	sources := []dashboardEventSource{
		{name: "throughput", interval: throughput.Interval, read: func(ctx context.Context) (any, error) {
			out, err := s.getDashboardThroughput(ctx, nil)
			if err != nil {
				return nil, err
			}
			return out.Body, nil
		}},
		{name: "connectivity", interval: dashboardStatusInterval, read: func(ctx context.Context) (any, error) {
			out, err := s.getDashboardConnectivity(ctx, nil)
			if err != nil {
				return nil, err
			}
			return out.Body, nil
		}},
		{name: "resources", interval: dashboardStatusInterval, read: func(ctx context.Context) (any, error) {
			out, err := s.getSystemInfo(ctx, nil)
			if err != nil {
				return nil, err
			}
			return out.Body, nil
		}},
		{name: "alerts", interval: dashboardAlertsInterval, read: func(ctx context.Context) (any, error) {
			out, err := s.getSystemAlerts(ctx, nil)
			if err != nil {
				return nil, err
			}
			return out.Body, nil
		}},
	}
	if s.Tailscale != nil {
		sources = append(sources, dashboardEventSource{name: "tailscale", interval: dashboardTailscaleInterval, read: func(ctx context.Context) (any, error) {
			out, err := s.getTailscale(ctx, nil)
			if err != nil {
				return nil, err
			}
			return out.Body, nil
		}})
	}
	s.events = newDashboardEventService(s.DaemonContext, dashboardStreamLimit, sources)
	operation := op("getDashboardEvents", http.MethodGet, "/api/v1/dashboard/events", "Live dashboard status", "dashboard", 503)
	operation.Description = "Server-sent events named connectivity, resources, throughput, alerts, tailscale (only when the daemon supervises the package) and source_unavailable. Event data is the corresponding dashboard GET body; source_unavailable data identifies the failed source."
	operation.Responses = map[string]*huma.Response{"200": {
		Description: "Live dashboard event stream",
		Content: map[string]*huma.MediaType{"text/event-stream": {Schema: &huma.Schema{
			Type: huma.TypeString, Description: "Named Server-Sent Events with UTF-8 JSON data. No event IDs or replay are provided.",
		}}},
	}}
	register(s, false, operation, a, s.openDashboardEvents)
}

func (s *server) openDashboardEvents(ctx context.Context, _ *struct{}) (*huma.StreamResponse, error) {
	sub, err := s.events.subscribe()
	if errors.Is(err, errStreamCapacity) {
		return nil, fail(http.StatusServiceUnavailable, "stream_capacity", "Dashboard event stream capacity is full")
	}
	if err != nil {
		return nil, fail(http.StatusServiceUnavailable, "stream_unavailable", "Dashboard event stream is stopping")
	}
	cred := authedOf(ctx)
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		defer sub.close()
		hctx.SetHeader("Content-Type", "text/event-stream")
		hctx.SetStatus(http.StatusOK)
		_, w := humago.Unwrap(hctx)
		heartbeat := time.NewTicker(dashboardHeartbeat)
		defer heartbeat.Stop()
		s.serveDashboardEvents(hctx.Context(), w, sub, cred, heartbeat.C, dashboardWriteTimeout)
	}}, nil
}

// serveDashboardEvents owns exactly one subscriber until the request, session,
// daemon or a bounded write ends it.
func (s *server) serveDashboardEvents(ctx context.Context, w http.ResponseWriter, sub *dashboardSubscription, cred authed, heartbeat <-chan time.Time, timeout time.Duration) {
	writer := dashboardSSEWriter{
		writer: w, controller: http.NewResponseController(w), timeout: timeout,
		sessionValid: func() bool { return s.stillValid(cred) },
	}
	if !writer.write([]byte(dashboardInitialComment)) {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.service.daemon.Done():
			return
		default:
		}
		if event, ok := sub.take(); ok {
			frame, err := marshalDashboardEvent(event)
			if err != nil || !writer.write(frame) {
				return
			}
			continue
		}
		select {
		case <-sub.wake:
		case <-heartbeat:
			if !writer.write([]byte(dashboardHeartbeatComment)) {
				return
			}
		case <-ctx.Done():
			return
		case <-sub.service.daemon.Done():
			return
		}
	}
}

func marshalDashboardEvent(event dashboardEvent) ([]byte, error) {
	data, err := json.Marshal(event.data)
	if err != nil {
		return nil, err
	}
	frame := make([]byte, 0, len(event.name)+len(data)+16)
	frame = append(frame, "event: "...)
	frame = append(frame, event.name...)
	frame = append(frame, "\ndata: "...)
	frame = append(frame, data...)
	frame = append(frame, '\n', '\n')
	return frame, nil
}

type dashboardSSEWriter struct {
	writer       http.ResponseWriter
	controller   *http.ResponseController
	timeout      time.Duration
	sessionValid func() bool
}

// write bounds the write AND flush, and clears the deadline only after a
// successful flush so an idle stream is not killed by its previous write.
func (w *dashboardSSEWriter) write(frame []byte) bool {
	if !w.sessionValid() {
		return false
	}
	if err := w.controller.SetWriteDeadline(time.Now().Add(w.timeout)); err != nil {
		return false
	}
	if err := writeAll(w.writer, frame); err != nil {
		return false
	}
	if err := w.controller.Flush(); err != nil {
		return false
	}
	return w.controller.SetWriteDeadline(time.Time{}) == nil
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}
