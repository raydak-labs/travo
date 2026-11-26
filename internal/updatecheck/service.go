// Package updatecheck discovers releases without downloading update packages.
package updatecheck

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

type Result struct {
	RunningVersion string     `json:"running_version"`
	LatestVersion  *string    `json:"latest_version" nullable:"true"`
	Status         string     `json:"status" enum:"disabled,deferred,no_release,source_unavailable,rate_limited,unavailable,available,up_to_date"`
	CheckedAt      *time.Time `json:"checked_at" nullable:"true"`
	DisabledReason *string    `json:"disabled_reason" nullable:"true" enum:"off,unstable_build" doc:"Why checks are disabled: off = release source set to off; unstable_build = the running build is not an X.Y.Z release; null otherwise"`
}

type attemptState struct {
	Source         string    `json:"source"`
	RunningVersion string    `json:"running_version"`
	AttemptedAt    time.Time `json:"attempted_at"`
}

type flight struct {
	done      chan struct{}
	result    Result
	throttled bool
}

type Service struct {
	daemon                     context.Context
	source, running, statePath string
	now                        func() time.Time
	transport                  http.RoundTripper

	mu      sync.Mutex
	current *flight
	last    *Result
}

func New(daemon context.Context, source, running, statePath string, now func() time.Time, transport http.RoundTripper) *Service {
	return &Service{daemon: daemon, source: source, running: running, statePath: statePath, now: now, transport: transport}
}

func (s *Service) Check(ctx context.Context, manual bool) Result {
	if reason := disabledReason(s.source, s.running); reason != "" {
		return Result{RunningVersion: s.running, Status: "disabled", DisabledReason: &reason}
	}
	for {
		s.mu.Lock()
		current := s.current
		if current == nil && ctx.Err() == nil && s.daemon.Err() == nil {
			current = &flight{done: make(chan struct{})}
			s.current = current
			go s.check(current, manual)
		}
		s.mu.Unlock()
		if current != nil {
			select {
			case <-ctx.Done():
			case <-s.daemon.Done():
			case <-current.done:
				if manual && current.throttled {
					// An automatic attempt suppressed by cooldown did no outbound work.
					continue
				}
				return current.result
			}
		}
		return Result{RunningVersion: s.running, Status: "unavailable"}
	}
}

func (s *Service) check(current *flight, manual bool) {
	result, attempted := s.attempt(manual)
	s.mu.Lock()
	defer s.mu.Unlock()
	current.throttled = result.Status == "deferred"
	if current.throttled && s.last != nil {
		result = *s.last
	}
	if attempted {
		s.last = &result
	}
	current.result = result
	s.current = nil
	close(current.done)
}

func (s *Service) attempt(manual bool) (Result, bool) {
	result := Result{RunningVersion: s.running, Status: "unavailable"}
	var previous attemptState
	err := atomicfile.ReadJSON(s.statePath, &previous)
	if !errors.Is(err, os.ErrNotExist) && (err != nil || !ValidSource(previous.Source) || previous.RunningVersion == "" || previous.AttemptedAt.IsZero()) {
		return result, false
	}
	now := s.now()
	if !manual && previous.Source == s.source && previous.RunningVersion == s.running && now.Sub(previous.AttemptedAt) < 24*time.Hour {
		result.Status = "deferred"
		return result, false
	}
	state := attemptState{s.source, s.running, now.UTC()}
	if s.daemon.Err() != nil || atomicfile.WriteJSON(s.statePath, state, 0600) != nil {
		return result, false
	}
	v, status := latest(s.daemon, s.transport, s.source)
	checked := s.now().UTC()
	result.CheckedAt = &checked
	result.Status = status
	if v != "" {
		result.LatestVersion = &v
		result.Status = "up_to_date"
		if newer(v, strings.TrimPrefix(s.running, "v")) {
			result.Status = "available"
		}
	}
	return result, true
}

// EligibleVersion reads the latest completed outbound check without initiating one.
func (s *Service) EligibleVersion() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil || s.last.Status != "available" || s.last.LatestVersion == nil {
		return "", false
	}
	return *s.last.LatestVersion, true
}

// disabledReason names why checks cannot run; "" when they can. An "off" source wins over a
// development build, since it is the operator's setting.
func disabledReason(source, running string) string {
	switch {
	case source == "off":
		return "off"
	case !stableVersion.MatchString(strings.TrimPrefix(running, "v")):
		return "unstable_build"
	}
	return ""
}
