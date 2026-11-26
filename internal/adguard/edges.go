package adguard

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"syscall"
	"time"
)

// AdminURL is AGH's admin API on loopback (ADR 0006); the gated proxy forwards here too.
const AdminURL = "http://" + LoopbackHTTP

// HTTPReady polls GET /control/status until it answers 200 or ctx ends. With `users: []`
// it answers without credentials (testdata/device/axt1800-25.12.6/adguard/http-status-code.txt).
func HTTPReady(client *http.Client, base string, every time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		var last error
		for {
			last = statusOK(ctx, client, base+"/control/status")
			if last == nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("%w (last: %v)", ctx.Err(), last)
			case <-time.After(every):
			}
		}
	}
}

func statusOK(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("control/status: HTTP %d", res.StatusCode)
	}
	return nil
}

// FreeBytes reports the space available to an unprivileged writer on the first path that
// exists (/overlay on OpenWrt, / elsewhere).
func FreeBytes(paths ...string) func() (uint64, error) {
	return func() (uint64, error) {
		var last error
		for _, p := range paths {
			var st syscall.Statfs_t
			if err := syscall.Statfs(p, &st); err != nil {
				last = err
				continue
			}
			return uint64(st.Bavail) * uint64(st.Bsize), nil
		}
		return 0, last
	}
}
