package updatecheck

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const DefaultSource = "blackdark-org/travo-greenfield"
const maxResponse = 4 << 20

var segment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var stableVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func ValidSource(source string) bool {
	if source == "off" {
		return true
	}
	owner, repo, ok := strings.Cut(source, "/")
	return ok && validSegment(owner) && validSegment(repo)
}

func validSegment(s string) bool {
	return s != "." && s != ".." && segment.MatchString(s)
}

type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

func (r release) eligibleVersion() string {
	v, tagged := strings.CutPrefix(r.Tag, "v")
	if !tagged || !stableVersion.MatchString(v) || r.Draft || r.Prerelease {
		return ""
	}
	counts := map[string]int{}
	for _, asset := range r.Assets {
		counts[asset.Name]++
	}
	for _, name := range []string{"travo-greenfield-" + v + "-aarch64.tar.gz", "checksums.txt", "checksums.txt.sig"} {
		if counts[name] != 1 {
			return ""
		}
	}
	return v
}

// newer compares canonical numeric components without converting them to machine integers.
func newer(a, b string) bool {
	partsA, partsB := strings.Split(a, "."), strings.Split(b, ".")
	for i, a := range partsA {
		b := partsB[i]
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		if a != b {
			return a > b
		}
	}
	return false
}

func latest(ctx context.Context, transport http.RoundTripper, source string) (string, string) {
	client := &http.Client{
		Transport:     transport,
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+source+"/releases?per_page=100", nil)
	if err != nil {
		return "", "unavailable"
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "travo-greenfield")
	res, err := client.Do(req)
	if err != nil {
		return "", "unavailable"
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusUnauthorized, http.StatusNotFound:
		return "", "source_unavailable"
	case http.StatusForbidden, http.StatusTooManyRequests:
		return "", "rate_limited"
	case http.StatusOK:
	default:
		return "", "unavailable"
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	var releases []release
	if err != nil || len(raw) > maxResponse || json.Unmarshal(raw, &releases) != nil || releases == nil {
		return "", "unavailable"
	}
	best := ""
	for _, r := range releases {
		if v := r.eligibleVersion(); v != "" && (best == "" || newer(v, best)) {
			best = v
		}
	}
	if best == "" {
		return "", "no_release"
	}
	return best, ""
}
