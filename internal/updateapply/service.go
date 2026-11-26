// Package updateapply activates verified releases and recovers interrupted updates.
package updateapply

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/updatestage"
)

// GuardName is the crash guard held while an update is active or needs recovery.
const GuardName = "update-in-progress"

var (
	ErrBusy        = errors.New("update busy")
	ErrStage       = errors.New("verified stage required")
	ErrUnavailable = errors.New("update unavailable")
	opID           = regexp.MustCompile("^[0-9a-f]{32}$")
)

type ApplyStatus struct {
	ID      string `json:"id,omitempty"`
	Version string `json:"version,omitempty"`
	State   string `json:"state" enum:"idle,applying,succeeded,rolled_back,aborted,recovery_required"`
}

var releaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type processIdentity struct {
	PID   int
	Start string
}

type journal struct {
	ApplyStatus
	Phase           string
	PreviousVersion string
	Endpoint        string
	Stopping        processIdentity
}

type Stage interface {
	WithVerified(string, func(string) error) error
}

type Router interface {
	UpdateSettings(context.Context) (listen, staticDir string, err error)
	StopUpdateService(context.Context) error
	StartUpdateService(context.Context) error
	UpdatePID(context.Context) (int, error)
}

type Service struct {
	layout
	guards  guard.Dir
	router  Router
	stage   Stage
	version string
}

func New(guards guard.Dir, router Router, stage Stage, version string) *Service {
	return &Service{realLayout(), guards, router, stage, version}
}

func (s *Service) acquire() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(s.lock), 0755); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(s.lock, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), s.lock)
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Sys().(*syscall.Stat_t).Uid != 0 {
		f.Close()
		return nil, ErrUnavailable
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return f, nil
}

func (s *Service) guarded() (bool, error) {
	_, err := os.Lstat(filepath.Join(s.guards.Path, GuardName))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) save(j journal) error {
	return atomicfile.WriteJSON(s.path("journal.json"), j, 0600)
}

func (s *Service) load() (journal, error) {
	var j journal
	err := atomicfile.ReadJSON(s.path("journal.json"), &j)
	if err != nil {
		return j, err
	}
	if !opID.MatchString(j.ID) || !releaseVersion.MatchString(j.Version) || j.PreviousVersion == "" || j.Stopping.PID < 0 {
		return j, errors.New("invalid update journal")
	}
	if j.Stopping.PID > 0 {
		if _, err := strconv.ParseUint(j.Stopping.Start, 10, 64); err != nil {
			return j, errors.New("invalid process identity")
		}
	} else if j.Stopping.Start != "" {
		return j, errors.New("invalid process identity")
	}
	const prefix = "http://"
	const suffix = "/api/health"
	authority, ok := strings.CutPrefix(j.Endpoint, prefix)
	if !ok {
		return j, errors.New("invalid health endpoint")
	}
	authority, ok = strings.CutSuffix(authority, suffix)
	if !ok {
		return j, errors.New("invalid health endpoint")
	}
	endpoint, err := healthEndpoint(authority)
	if err != nil || endpoint != j.Endpoint {
		return j, errors.New("invalid health endpoint")
	}
	expected := j.Phase
	switch j.Phase {
	case "awaiting_ack", "switching":
		expected = "applying"
	case "succeeded", "rolled_back", "aborted", "recovery_required":
	default:
		return j, errors.New("invalid update phase")
	}
	if expected != j.State {
		return j, errors.New("invalid update state")
	}
	return j, nil
}

func (s *Service) Status() ApplyStatus {
	j, err := s.load()
	if err == nil {
		return j.ApplyStatus
	}
	present, guardErr := s.guarded()
	if errors.Is(err, os.ErrNotExist) && guardErr == nil && !present {
		return ApplyStatus{State: "idle"}
	}
	return ApplyStatus{State: "recovery_required"}
}

func healthEndpoint(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", err
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return "", ErrUnavailable
	}
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	default:
		if net.ParseIP(host) == nil {
			return "", ErrUnavailable
		}
	}
	return "http://" + net.JoinHostPort(host, port) + "/api/health", nil
}

type Accepted struct {
	Status ApplyStatus
	ack    *os.File
}

func (a *Accepted) Finish(flushed bool) {
	if a.ack == nil {
		return
	}
	if flushed {
		_, _ = a.ack.Write([]byte{1})
	}
	_ = a.ack.Close()
}

func (s *Service) Begin(ctx context.Context, version string) (*Accepted, error) {
	if s.stage == nil {
		return nil, ErrStage
	}
	lock, err := s.acquire()
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	present, err := s.guarded()
	if err != nil {
		return nil, err
	}
	if present {
		return nil, ErrBusy
	}
	if _, err := os.Lstat(s.root); err == nil {
		if err := s.cleanScratch(); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listen, staticDir, err := s.router.UpdateSettings(ctx)
	if err != nil || staticDir != s.ui {
		return nil, ErrUnavailable
	}
	endpoint, err := healthEndpoint(listen)
	if err != nil {
		return nil, ErrUnavailable
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return nil, err
	}
	j := journal{ApplyStatus: ApplyStatus{hex.EncodeToString(id), version, "applying"}, Phase: "awaiting_ack", PreviousVersion: s.version, Endpoint: endpoint}
	defer func() {
		if present, err := s.guarded(); err == nil && !present {
			// No installed files can have changed before the guard.
			_ = s.cleanScratch()
		}
	}()
	err = s.stage.WithVerified(version, func(path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return s.prepare(path, exe)
	})
	if errors.Is(err, updatestage.ErrChanged) {
		return nil, ErrStage
	}
	if err != nil {
		return nil, err
	}
	if err = s.save(j); err != nil {
		return nil, err
	}
	if err = s.guards.Acquire(GuardName); err != nil {
		return nil, err
	}
	if err = syncDir(s.guards.Path); err != nil {
		return nil, err
	}
	ack, err := s.spawn(lock, true)
	if err != nil {
		_ = s.finalize(j, "aborted")
		return nil, err
	}
	return &Accepted{Status: j.ApplyStatus, ack: ack}, nil
}

func (s *Service) spawn(lock *os.File, activation bool) (*os.File, error) {
	var reader, writer *os.File
	args := []string{"--update-worker"}
	if activation {
		var err error
		reader, writer, err = os.Pipe()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		args = append(args, "--update-await-ack")
	}
	cmd := exec.Command(s.path("recovery"), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{lock}
	if reader != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, reader)
	}
	if err := cmd.Start(); err != nil {
		if writer != nil {
			writer.Close()
		}
		return nil, err
	}
	go func() { _ = cmd.Wait() }()
	return writer, nil
}

// Recover schedules recovery without waiting for the replacement listener.
func (s *Service) Recover() (ApplyStatus, error) {
	lock, err := s.acquire()
	if err != nil {
		return ApplyStatus{}, err
	}
	defer lock.Close()
	present, err := s.guarded()
	if err != nil {
		return ApplyStatus{}, err
	}
	if !present {
		if j, err := s.load(); err == nil && j.Phase == "awaiting_ack" {
			if err := s.finalize(j, "aborted"); err != nil {
				return ApplyStatus{}, err
			}
		}
		return s.Status(), nil
	}
	j, err := s.load()
	if err != nil {
		return ApplyStatus{}, ErrUnavailable
	}
	_, err = s.spawn(lock, false)
	return j.ApplyStatus, err
}

// RecoverLocal executes recovery in the caller with the shared lock held.
func (s *Service) RecoverLocal() error {
	lock, err := s.acquire()
	if err != nil {
		return err
	}
	defer lock.Close()
	return s.work(nil)
}

// Work is invoked only by the trusted detached executable with inherited descriptors.
func (s *Service) Work(activation bool) error {
	lock := os.NewFile(3, "update lock")
	if lock == nil {
		return ErrUnavailable
	}
	defer lock.Close()
	syscall.CloseOnExec(3)
	st, err := lock.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(s.lock)
	if err != nil || !os.SameFile(st, current) {
		return ErrUnavailable
	}
	// The inherited open description already holds the lock. This also checks
	// that a caller cannot supply an unrelated descriptor while an update runs.
	if err = syscall.Flock(3, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	if !activation {
		return s.work(nil)
	}
	if err := makeAckNonblocking(); err != nil {
		return err
	}
	ack := os.NewFile(4, "update acknowledgement")
	if ack == nil {
		return ErrUnavailable
	}
	defer ack.Close()
	return s.work(ack)
}

func makeAckNonblocking() error {
	syscall.CloseOnExec(4)
	return syscall.SetNonblock(4, true)
}

func acknowledge(ack *os.File) bool {
	if err := ack.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return false
	}
	var b [1]byte
	_, err := io.ReadFull(ack, b[:])
	return err == nil && b[0] == 1
}

func (s *Service) work(ack *os.File) error {
	present, err := s.guarded()
	if err != nil || !present {
		return err
	}
	j, err := s.load()
	if err != nil {
		return err
	}
	switch j.Phase {
	case "succeeded", "rolled_back", "aborted":
		return s.finalize(j, j.Phase)
	case "awaiting_ack":
		if ack == nil || !acknowledge(ack) {
			return s.finalize(j, "aborted")
		}
		j.Phase = "switching"
		if err = s.save(j); err != nil {
			return err
		}
		if err = s.switchAssets(&j, "next", j.Version); err == nil {
			return s.finalize(j, "succeeded")
		}
		slog.Warn("update activation failed, restoring previous release", "err", err)
	}
	if err := s.switchAssets(&j, "rollback", j.PreviousVersion); err != nil {
		j.Phase, j.State = "recovery_required", "recovery_required"
		return errors.Join(fmt.Errorf("update rollback: %w", err), s.save(j))
	}
	return s.finalize(j, "rolled_back")
}

func (s *Service) finalize(j journal, result string) error {
	j.Phase, j.State = result, result
	if err := s.save(j); err != nil {
		return err
	}
	if result == "succeeded" {
		if err := s.promote(); err != nil {
			return err
		}
	}
	// Keep the saved worker until the guard is released so finalization remains retryable.
	if err := s.guards.Release(GuardName); err != nil {
		return err
	}
	if err := syncDir(s.guards.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.cleanScratch()
}
