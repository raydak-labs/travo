package api

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func (s *dashboardEventService) currentRun() *dashboardProducerRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run
}

func waitProducers(t *testing.T, run *dashboardProducerRun) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		run.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producers did not stop after the last unsubscribe")
	}
}

func openEventStream(t *testing.T, base, cookie string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/dashboard/events", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("open stream: %d %s", resp.StatusCode, body)
	}
	return resp
}

func readSSEFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	if frame := readSSEFrameAllowEOF(t, reader); frame != "" {
		return frame
	}
	t.Fatal("unexpected EOF")
	return ""
}

func readSSEFrameAllowEOF(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	type result struct {
		frame string
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		var frame strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				resultCh <- result{frame: frame.String(), err: err}
				return
			}
			frame.WriteString(line)
			if line == "\n" || line == "\r\n" {
				resultCh <- result{frame: frame.String()}
				return
			}
		}
	}()
	select {
	case result := <-resultCh:
		if result.err != nil {
			return ""
		}
		return result.frame
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for an SSE frame")
		return ""
	}
}

func parseSSEEvent(t *testing.T, frame string) (string, map[string]any) {
	t.Helper()
	var name string
	var data strings.Builder
	for line := range strings.SplitSeq(strings.TrimSuffix(frame, "\n\n"), "\n") {
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			name = value
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			data.WriteString(value)
		}
	}
	var body map[string]any
	if name == "" || json.Unmarshal([]byte(data.String()), &body) != nil {
		t.Fatalf("bad SSE event %q", frame)
	}
	return name, body
}

func jsonString(v any) string {
	body, _ := json.Marshal(v)
	return string(body)
}
