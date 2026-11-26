package adguard

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Memory caps for a 418 MB router whose kernel and WiFi leave ~80 MB available
// (device-facts §11). AGH grew ~0.8 MB/h and was OOM-killed at 116 MB anon-rss; then the
// router thrashed. The caps are ceilings: a smaller value a user picked is kept. MemLimit is
// reconciled on every tick; the YAML caps apply once per router (CapsMarker, migrate.go).
const (
	// MemLimit is AGH's Go soft memory limit (GOMEMLIMIT via the init script's memlimit,
	// bytes). AGH's Go memory (RssAnon) is ~54 MB with the seed's filter list; 64 MiB
	// leaves it room while the GC starts reclaiming long before the ~80 MB budget is gone.
	MemLimit = 64 << 20
	// QueryLogMemory is querylog.size_memory: entries buffered in RAM before a flush.
	QueryLogMemory = 100
	// QueryLogInterval is querylog.interval. The file lives in /var/lib/adguardhome, which is
	// tmpfs (RAM) on OpenWrt: 90 days of queries would grow in memory without bound.
	QueryLogInterval = 6 * time.Hour
	// StatsInterval is statistics.interval (stats.db, also in tmpfs); AGH's smallest choice.
	StatsInterval = 24 * time.Hour
	// CacheSize is dns.cache_size in bytes.
	CacheSize = 4 << 20
)

// UCI names of the package's init script options (testdata/device/axt1800-25.12.6/adguard/initd.txt).
const (
	UCIConfig      = "adguardhome"
	UCISection     = "config"
	MemLimitOption = "memlimit"
)

// yamlCap is one YAML scalar Travo caps: key inside the top-level block, the value Travo
// writes, and whether the current value exceeds the cap.
type yamlCap struct {
	block, key, value string
	over              func(string) bool
}

var yamlCaps = []yamlCap{
	{"querylog:", "size_memory", strconv.Itoa(QueryLogMemory), uintOver(QueryLogMemory)},
	{"querylog:", "interval", "6h", durationOver(QueryLogInterval)},
	{"statistics:", "interval", "1d", durationOver(StatsInterval)},
	{"dns:", "cache_size", strconv.Itoa(CacheSize), uintOver(CacheSize)},
}

// Limits caps the memory-relevant keys of AGH's YAML. Every other byte stays as it was.
// It is total: a missing block or key, or a shape it cannot read, is left alone (AGH's own
// default applies), so the caps never block the loopback migration.
func Limits(yaml []byte) (out []byte, changed bool) {
	s := string(yaml)
	if strings.ContainsAny(s, "\r\t") {
		return yaml, false
	}
	lines := strings.Split(s, "\n")
	for _, c := range yamlCaps {
		b, err := block(lines, c.block)
		if err != nil {
			continue
		}
		sp, val, err := scalar(lines, b, c.key)
		if err != nil || !c.over(val) {
			continue
		}
		lines[sp.start] = "  " + c.key + ": " + c.value
		changed = true
	}
	if !changed {
		return yaml, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

func uintOver(limit uint64) func(string) bool {
	return func(v string) bool {
		n, err := strconv.ParseUint(v, 10, 64)
		return err != nil || n > limit
	}
}

func durationOver(limit time.Duration) func(string) bool {
	return func(v string) bool {
		d, ok := parseAGHDuration(v)
		return !ok || d > limit
	}
}

// parseAGHDuration reads AGH's durations: Go syntax ("2160h", "6h0m0s") plus whole days
// ("90d"), the form 0.107.79 writes (testdata/device/axt1800-25.12.6/adguard/adguardhome.yaml).
func parseAGHDuration(v string) (time.Duration, bool) {
	if days, ok := strings.CutSuffix(v, "d"); ok {
		n, err := strconv.ParseUint(days, 10, 32)
		return time.Duration(n) * 24 * time.Hour, err == nil
	}
	d, err := time.ParseDuration(v)
	return d, err == nil
}

// MemLimitTarget is pure: the adguardhome UCI sections in, the memlimit to write out.
// ok is false when the section is missing or its memlimit is already within the cap
// (0, absent or unparseable means unlimited and is capped).
func MemLimitTarget(sections []device.Section) (value string, ok bool) {
	for _, s := range sections {
		if s.Name != UCISection || s.Type != UCIConfig {
			continue
		}
		n, err := strconv.ParseUint(s.Options[MemLimitOption], 10, 64)
		if err == nil && n > 0 && n <= MemLimit {
			return "", false
		}
		return strconv.Itoa(MemLimit), true
	}
	return "", false
}

// applyMemLimit writes the memlimit through the transaction module. It does not restart
// AGH: the caller restarts once for this and the YAML caps together.
func (s *Service) applyMemLimit(ctx context.Context) (changed bool, err error) {
	if s.UCI == nil || s.Txns == nil {
		return false, nil
	}
	sections, err := s.UCI.Show(ctx, UCIConfig)
	if errors.Is(err, device.ErrNotFound) {
		return false, nil // no init script options to set
	}
	if err != nil {
		return false, err
	}
	v, ok := MemLimitTarget(sections)
	if !ok {
		return false, nil
	}
	err = s.Txns.Apply(ctx, []string{UCIConfig}, func(ctx context.Context) error {
		return s.UCI.Set(ctx, UCIConfig, UCISection, MemLimitOption, v)
	})
	return err == nil, err
}

// restart applies a changed init script option (GOMEMLIMIT is read at process start).
func (s *Service) restart(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.idle(); err != nil {
		return err
	}
	if err := s.Services.ServiceAction(ctx, Package, "restart"); err != nil {
		return fmt.Errorf("%s restart: %w", Package, err)
	}
	return s.waitReady(ctx)
}
