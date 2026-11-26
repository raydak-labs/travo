package updateapply

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (s *Service) switchAssets(j *journal, slot, version string) error {
	stop, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err := s.quiesce(stop, j)
	cancel()
	if err != nil {
		return err
	}
	if err = s.installAssets(slot); err != nil {
		return err
	}
	start, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = s.router.StartUpdateService(start)
	cancel()
	if err != nil {
		return err
	}
	return s.waitHealthy(j.Endpoint, version)
}

func (s *Service) waitHealthy(endpoint, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := &http.Client{
		Timeout:       2 * time.Second,
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		before := s.runningPID(ctx)
		if before > 0 && healthyVersion(ctx, client, endpoint, version) && s.runningPID(ctx) == before {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("updated service did not pass verification")
		case <-ticker.C:
		}
	}
}

func (s *Service) runningPID(ctx context.Context) int {
	pid, err := s.router.UpdatePID(ctx)
	if err != nil || pid <= 0 {
		return 0
	}
	running, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return 0
	}
	installed, err := regular(s.binary)
	if err != nil || !os.SameFile(running, installed) {
		return 0
	}
	return pid
}

func healthyVersion(ctx context.Context, client *http.Client, endpoint, version string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false
	}
	var body struct {
		Status  string
		Version string
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&body) == nil && body.Status == "ok" && body.Version == version
}

func processAt(pid int) (processIdentity, error) {
	return readProcessAt(pid, os.ReadFile)
}

func readProcessAt(pid int, readFile func(string) ([]byte, error)) (processIdentity, error) {
	if pid == 0 {
		return processIdentity{}, nil
	}
	raw, err := readFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	// Procfs may return ESRCH if the task exits after the file is opened.
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
		return processIdentity{}, nil
	}
	if err != nil {
		return processIdentity{}, err
	}
	// The comm field is parenthesized and may itself contain spaces or parentheses.
	end := strings.LastIndexByte(string(raw), ')')
	if end < 0 {
		return processIdentity{}, errors.New("invalid process stat")
	}
	fields := strings.Fields(string(raw[end+1:]))
	if len(fields) < 20 {
		return processIdentity{}, errors.New("short process stat")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return processIdentity{}, err
	}
	if fields[0] == "Z" || fields[0] == "X" {
		return processIdentity{}, nil
	}
	return processIdentity{pid, fields[19]}, nil
}

func waitProcess(ctx context.Context, process processIdentity) error {
	if process.PID == 0 {
		return nil
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		actual, err := processAt(process.PID)
		if err != nil || actual != process {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func (s *Service) quiesce(ctx context.Context, j *journal) error {
	pid, err := s.router.UpdatePID(ctx)
	if err != nil {
		return err
	}
	current, err := processAt(pid)
	if err != nil {
		return err
	}
	if j.Stopping.PID > 0 && j.Stopping != current {
		if err := waitProcess(ctx, j.Stopping); err != nil {
			return err
		}
	}
	j.Stopping = current
	if err := s.save(*j); err != nil {
		return err
	}
	if err := s.router.StopUpdateService(ctx); err != nil {
		return err
	}
	if err := waitProcess(ctx, current); err != nil {
		return err
	}
	// service list can keep the instance after the task has exited or respawned.
	if err := awaitInstanceExit(ctx, s.router, current, processAt); err != nil {
		return err
	}
	j.Stopping = processIdentity{}
	return s.save(*j)
}

func awaitInstanceExit(ctx context.Context, router Router, stopped processIdentity, lookup func(int) (processIdentity, error)) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		remaining, err := router.UpdatePID(ctx)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return nil
		}
		actual, err := lookup(remaining)
		if err != nil {
			return err
		}
		if actual.PID != 0 && actual != stopped {
			if err := router.StopUpdateService(ctx); err != nil {
				return err
			}
			if err := waitProcess(ctx, actual); err != nil {
				return err
			}
			stopped = actual
			continue
		}
		select {
		case <-ctx.Done():
			return errors.New("procd instance still running")
		case <-tick.C:
		}
	}
}
