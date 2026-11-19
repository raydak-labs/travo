package osupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

var (
	// ErrOffline means no route or the release server was unreachable.
	ErrOffline = errors.New("osupdate: offline")
	// ErrFailed means the router or the release index answered something unusable.
	ErrFailed = errors.New("osupdate: check failed")
)

// IndexLimit caps the release index download (the real one is about 2 KB).
const IndexLimit = 256 << 10

// OpenWrtCheck is the last check, kept in memory only.
type OpenWrtCheck struct {
	CheckedAt *time.Time `json:"checked_at" nullable:"true" doc:"When the last check ran; null before the first check since the daemon started"`
	Target    string     `json:"target" doc:"Firmware target, e.g. qualcommax/ipq60xx"`
	Releases
	PackageManager string           `json:"package_manager" doc:"apk or opkg"`
	ListsRefreshed bool             `json:"lists_refreshed" doc:"The package lists were refreshed before listing (the user asked)"`
	PackagesRead   bool             `json:"packages_read" doc:"False when listing upgradable packages failed"`
	Packages       []pkgmgr.Upgrade `json:"packages" nullable:"false"`
}

// Packages is the package manager slice the check needs.
type Packages interface {
	Name() string
	Update(context.Context) error
	pkgmgr.UpgradeLister
}

// Service runs one check at a time and remembers the last result.
type Service struct {
	Ubus     device.Ubus
	Routes   device.Routes
	Packages Packages
	// Fetch downloads url; network failures wrap ErrOffline.
	Fetch func(ctx context.Context, url string) ([]byte, error)
	Now   func() time.Time

	check sync.Mutex
	mu    sync.Mutex
	last  OpenWrtCheck
}

// Last returns the last result; an empty one before the first check.
func (s *Service) Last() OpenWrtCheck {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.last
	if out.Packages == nil {
		out.Packages = []pkgmgr.Upgrade{}
	}
	return out
}

// Check fetches the release index for the running target and lists upgradable packages.
// refresh updates the package lists first (they live in /tmp); it is the user's choice.
func (s *Service) Check(ctx context.Context, refresh bool) (OpenWrtCheck, error) {
	s.check.Lock()
	defer s.check.Unlock()
	raw, err := s.Ubus.Call(ctx, "system", "board", nil)
	if err != nil {
		return OpenWrtCheck{}, errors.Join(ErrFailed, err)
	}
	board, url, err := parseBoard(raw)
	if err != nil {
		return OpenWrtCheck{}, errors.Join(ErrFailed, err)
	}
	if up, err := s.Routes.HasDefaultRoute(ctx); err == nil && !up {
		return OpenWrtCheck{}, ErrOffline
	}
	body, err := s.Fetch(ctx, url+".versions.json")
	if err != nil {
		return OpenWrtCheck{}, err
	}
	ix, err := ParseIndex(body)
	if err != nil {
		return OpenWrtCheck{}, errors.Join(ErrFailed, err)
	}
	now := s.Now().UTC()
	res := OpenWrtCheck{CheckedAt: &now, Target: board.Target, Releases: Compare(board.Release, ix), PackageManager: s.Packages.Name(), Packages: []pkgmgr.Upgrade{}}
	if refresh {
		if err := s.Packages.Update(ctx); err != nil {
			slog.Warn("package lists refresh", "err", err) // best effort (invariants §13)
		} else {
			res.ListsRefreshed = true
		}
	}
	if ups, err := s.Packages.Upgradable(ctx); err != nil {
		slog.Warn("upgradable packages", "err", err)
	} else {
		res.PackagesRead, res.Packages = true, ups
	}
	s.mu.Lock()
	s.last = res
	s.mu.Unlock()
	return res, nil
}

// parseBoard reads the release and the firmware server, which must be an https URL.
func parseBoard(raw []byte) (facts.Board, string, error) {
	board, err := facts.ParseBoard(raw)
	if err != nil {
		return facts.Board{}, "", err
	}
	url, err := facts.FirmwareURL(raw)
	if err != nil {
		return facts.Board{}, "", err
	}
	if !strings.HasPrefix(url, "https://") || strings.ContainsAny(url, " \n?#") {
		return facts.Board{}, "", fmt.Errorf("firmware_url %q is not an https URL", url)
	}
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return board, url, nil
}

// HTTPFetch downloads with a bounded time and size. Transport errors are ErrOffline.
func HTTPFetch(client *http.Client) func(context.Context, string) ([]byte, error) {
	return func(ctx context.Context, url string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, errors.Join(ErrFailed, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.Join(ErrOffline, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: release index answered %d", ErrFailed, resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, IndexLimit+1))
		if err != nil {
			return nil, errors.Join(ErrOffline, err)
		}
		if len(b) > IndexLimit {
			return nil, fmt.Errorf("%w: release index too large", ErrFailed)
		}
		return b, nil
	}
}
