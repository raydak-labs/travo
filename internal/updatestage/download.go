package updatestage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	archiveLimit   = 16 << 20
	manifestLimit  = 64 << 10
	signatureLimit = 1 << 10
)

func ReleaseClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport, CheckRedirect: checkRedirect}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	u := req.URL
	if len(via) > 3 || u.Scheme != "https" || u.User != nil ||
		(u.Port() != "" && u.Port() != "443") ||
		(u.Hostname() != "github.com" && u.Hostname() != "release-assets.githubusercontent.com") {
		return errors.New("release asset redirect refused")
	}
	// The CDN signs the redirect URL. Never forward headers or referers between hosts.
	req.Header.Del("Referer")
	return nil
}

func download(ctx context.Context, client *http.Client, url, destination string, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.ContentLength > limit {
		return errors.New("release asset unavailable or oversized")
	}
	return writeBounded(destination, res.Body, limit)
}

func writeBounded(destination string, source io.Reader, limit int64) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(file, io.LimitReader(source, limit+1))
	err = errors.Join(copyErr, file.Close())
	if n > limit {
		return errors.New("release asset exceeds size limit")
	}
	return err
}

func (s *Service) prepare(ctx context.Context, dir, version string) error {
	base := fmt.Sprintf("https://github.com/%s/releases/download/v%s/", s.source, version)
	manifest, sig := filepath.Join(dir, "checksums.txt"), filepath.Join(dir, "checksums.txt.sig")
	if err := download(ctx, s.client, base+"checksums.txt", manifest, manifestLimit); err != nil {
		return err
	}
	if err := download(ctx, s.client, base+"checksums.txt.sig", sig, signatureLimit); err != nil {
		return err
	}
	key := filepath.Join(dir, "trusted.pub")
	if err := os.WriteFile(key, []byte(s.key), 0600); err != nil {
		return err
	}
	if err := s.verifier.VerifyRelease(ctx, manifest, sig, key); err != nil {
		return err
	}
	signed, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	archive := filepath.Join(dir, "release.tar.gz")
	if err := download(ctx, s.client, base+"travo-greenfield-"+version+"-aarch64.tar.gz", archive, archiveLimit); err != nil {
		return err
	}
	if err := extract(ctx, archive, filepath.Join(dir, "payload"), signed, version); err != nil {
		return err
	}
	return os.Remove(archive)
}
