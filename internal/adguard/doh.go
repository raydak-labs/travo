package adguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// DNS over HTTPS is AdGuard Home's upstream (ADR 0010 "DNS over HTTPS"): the `adguard`
// layer carries it, so the captive layer outranks it and the liveness probe fails it open.

var (
	// ErrDoHInvalid: not an https:// URL with a host and no credentials.
	ErrDoHInvalid = errors.New("the DNS over HTTPS URL must be https://host/path without credentials")
	// ErrDoHUnreachable: AGH's own upstream test failed; nothing was changed.
	ErrDoHUnreachable = errors.New("AdGuard Home could not resolve through this DNS over HTTPS server; nothing was changed")
	// ErrDoHNoChange: the upstream is already exactly this URL.
	ErrDoHNoChange = errors.New("AdGuard Home already uses this upstream")
	// ErrAdminAPI: AGH's loopback admin API did not answer as expected (not running, restarting).
	ErrAdminAPI = errors.New("AdGuard Home's admin API is unavailable")
)

// DoHView is the GET body: AGH's upstream list as stored.
type DoHView struct {
	Upstreams []string `json:"upstreams" doc:"AdGuard Home upstream_dns, in order"`
	Encrypted bool     `json:"encrypted" doc:"Every upstream is DNS over HTTPS, TLS or QUIC"`
}

// DoHRequest replaces the upstream list with one DoH server.
type DoHRequest struct {
	URL string `json:"url" maxLength:"512" doc:"https:// DNS over HTTPS endpoint, e.g. https://dns10.quad9.net/dns-query"`
}

// Upstreams reads and changes AGH's upstream through its loopback admin API (ADR 0006).
type Upstreams struct {
	Client *http.Client
	Base   string // AdminURL
	// Idle refuses while an install or config swap holds its guard (ErrBusy).
	Idle func() error
}

// dnsInfo is the slice of GET /control/dns_info Travo reads (testdata adguard/dns-info.json).
type dnsInfo struct {
	UpstreamDNS  []string `json:"upstream_dns"`
	BootstrapDNS []string `json:"bootstrap_dns"`
}

// ValidDoH accepts an https URL with a host and no userinfo, query or fragment.
func ValidDoH(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.Hostname() != "" && u.User == nil &&
		u.RawQuery == "" && u.Fragment == "" && !strings.ContainsAny(raw, " \t\r\n") && len(raw) <= 512
}

// Encrypted: every upstream uses an encrypted transport AGH supports.
func Encrypted(ups []string) bool {
	return len(ups) > 0 && !slices.ContainsFunc(ups, func(u string) bool {
		for _, p := range []string{"https://", "tls://", "quic://", "h3://", "sdns://"} {
			if strings.HasPrefix(u, p) {
				return false
			}
		}
		return true
	})
}

// UpstreamOK reads AGH's test_upstream_dns reply: a map keyed by the normalised upstream
// (`https://host:443/path`, testdata adguard/test-upstream-dns-*.json) to "OK" or an error.
func UpstreamOK(body []byte) bool {
	var res map[string]string
	if json.Unmarshal(body, &res) != nil || len(res) == 0 {
		return false
	}
	for _, v := range res {
		if v != "OK" {
			return false
		}
	}
	return true
}

// Get never mutates.
func (u *Upstreams) Get(ctx context.Context) (DoHView, error) {
	info, err := u.info(ctx)
	if err != nil {
		return DoHView{}, err
	}
	ups := info.UpstreamDNS
	if ups == nil {
		ups = []string{}
	}
	return DoHView{Upstreams: ups, Encrypted: Encrypted(ups)}, nil
}

// Set tests the URL with AGH's bootstrap resolvers, then writes it as the only upstream and
// reads it back. A failed test changes nothing.
func (u *Upstreams) Set(ctx context.Context, in DoHRequest) (DoHView, error) {
	if !ValidDoH(in.URL) {
		return DoHView{}, ErrDoHInvalid
	}
	if u.Idle != nil {
		if err := u.Idle(); err != nil {
			return DoHView{}, err
		}
	}
	info, err := u.info(ctx)
	if err != nil {
		return DoHView{}, err
	}
	if slices.Equal(info.UpstreamDNS, []string{in.URL}) {
		return DoHView{}, ErrDoHNoChange
	}
	test, err := u.post(ctx, "/control/test_upstream_dns", map[string]any{
		"upstream_dns": []string{in.URL}, "bootstrap_dns": info.BootstrapDNS,
		"fallback_dns": []string{}, "private_upstream": []string{},
	})
	if err != nil {
		return DoHView{}, err
	}
	if !UpstreamOK(test) {
		return DoHView{}, ErrDoHUnreachable
	}
	if _, err := u.post(ctx, "/control/dns_config", map[string]any{"upstream_dns": []string{in.URL}}); err != nil {
		return DoHView{}, err
	}
	v, err := u.Get(ctx)
	if err == nil && !slices.Equal(v.Upstreams, []string{in.URL}) {
		err = fmt.Errorf("%w: upstream not stored", ErrAdminAPI)
	}
	return v, err
}

func (u *Upstreams) info(ctx context.Context) (dnsInfo, error) {
	var info dnsInfo
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.Base+"/control/dns_info", nil)
	if err != nil {
		return info, err
	}
	body, err := u.do(req)
	if err == nil && json.Unmarshal(body, &info) != nil {
		err = fmt.Errorf("%w: dns_info is not JSON", ErrAdminAPI)
	}
	return info, err
}

func (u *Upstreams) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.Base+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return u.do(req)
}

func (u *Upstreams) do(req *http.Request) ([]byte, error) {
	res, err := u.Client.Do(req)
	if err != nil {
		return nil, errors.Join(ErrAdminAPI, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return nil, errors.Join(ErrAdminAPI, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s HTTP %d", ErrAdminAPI, req.URL.Path, res.StatusCode)
	}
	return body, nil
}
