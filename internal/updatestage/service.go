// Package updatestage downloads and verifies updates without applying them.
package updatestage

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/updatecheck"
)

const (
	Root = "/tmp/travo-greenfield-update"
	ttl  = 30 * time.Minute
)

var (
	ErrBusy        = errors.New("release stage busy")
	ErrChanged     = errors.New("eligible release changed")
	ErrUnavailable = errors.New("release stage unavailable")
	stageName      = regexp.MustCompile(`^stage-[0-9]+$`)
)

type StageStatus struct {
	Version *string `json:"version" nullable:"true"`
	Status  string  `json:"status" enum:"idle,verifying,verified"`
}

type stage struct {
	version, dir string
	expires      time.Time
}

type Service struct {
	ctx               context.Context
	cancel            context.CancelFunc
	root, source, key string
	lock              *os.File
	client            *http.Client
	verifier          device.ReleaseVerifier
	eligible          func() (string, bool)
	now               func() time.Time

	mu        sync.Mutex
	closed    bool
	active    *stage
	candidate string
	retired   []string
	attempt   sync.WaitGroup
}

func Open(ctx context.Context, root, source, key string, client *http.Client, verifier device.ReleaseVerifier, eligible func() (string, bool), now func() time.Time) (*Service, error) {
	if source == "off" || !updatecheck.ValidSource(source) || key == "" || verifier == nil {
		return nil, ErrUnavailable
	}
	lock, err := lockRoot(root)
	if err != nil {
		return nil, err
	}
	if err := cleanStale(root); err != nil {
		lock.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Service{ctx: ctx, cancel: cancel, root: root, source: source, key: key, lock: lock, client: client, verifier: verifier, eligible: eligible, now: now}, nil
}

func (s *Service) Status() StageStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.candidate != "":
		return StageStatus{Version: versionPointer(s.candidate), Status: "verifying"}
	case s.active != nil && s.now().Before(s.active.expires):
		return StageStatus{Version: versionPointer(s.active.version), Status: "verified"}
	default:
		return StageStatus{Status: "idle"}
	}
}

func versionPointer(version string) *string { return &version }

func (s *Service) admit(version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil {
		return ErrUnavailable
	}
	if s.candidate != "" {
		return ErrBusy
	}
	eligible, ok := s.eligible()
	if !ok || version != eligible || !stableVersion.MatchString(version) {
		return ErrChanged
	}
	s.candidate = version
	s.attempt.Add(1)
	return nil
}

var stableVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func (s *Service) Stage(request context.Context, version string) (StageStatus, error) {
	if err := s.admit(version); err != nil {
		return StageStatus{}, err
	}
	defer s.attempt.Done()
	ctx, cancel := context.WithTimeout(s.ctx, 120*time.Second)
	defer cancel()
	stop := context.AfterFunc(request, cancel)
	defer stop()
	if request.Err() != nil {
		cancel()
	}
	dir, err := os.MkdirTemp(s.root, "stage-")
	if err == nil {
		err = s.prepare(ctx, dir, version)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.candidate = ""
	if err = errors.Join(err, ctx.Err(), request.Err()); err != nil {
		if dir != "" {
			err = errors.Join(err, os.RemoveAll(dir))
		}
		return StageStatus{}, err
	}
	if s.active != nil {
		s.retired = append(s.retired, s.active.dir)
	}
	s.active = &stage{version: version, dir: dir, expires: s.now().Add(ttl)}
	_ = s.cleanRetired()
	return StageStatus{Version: &version, Status: "verified"}, nil
}

// Sweep removes expired verified stages. Partial attempts remain owned by Stage.
func (s *Service) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && !s.now().Before(s.active.expires) {
		s.retired = append(s.retired, s.active.dir)
		s.active = nil
	}
	_ = s.cleanRetired()
}

func (s *Service) cleanRetired() error {
	var errs error
	remaining := s.retired[:0]
	for _, dir := range s.retired {
		if err := os.RemoveAll(dir); err != nil {
			errs = errors.Join(errs, err)
			remaining = append(remaining, dir)
		}
	}
	s.retired = remaining
	return errs
}

func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.attempt.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	if s.active != nil {
		s.retired = append(s.retired, s.active.dir)
		s.active = nil
	}
	err := errors.Join(s.cleanRetired(), s.lock.Close())
	s.lock = nil
	return err
}

// WithVerified holds the stage lease while fn consumes a private copy of its
// payload. The stage path belongs to this service and cannot outlive fn.
func (s *Service) WithVerified(version string, fn func(string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil || s.active == nil || s.active.version != version || !s.now().Before(s.active.expires) {
		return ErrChanged
	}
	return fn(filepath.Join(s.active.dir, "payload"))
}
