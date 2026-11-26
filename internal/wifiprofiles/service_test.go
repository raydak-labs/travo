package wifiprofiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

const secret = "sentinel-secret-1234"

type recordingJoiner struct {
	mu    sync.Mutex
	calls []wifi.JoinRequest
	join  func(wifi.JoinRequest) (txn.Status, error)
}

func (j *recordingJoiner) Join(_ context.Context, req wifi.JoinRequest) (txn.Status, error) {
	j.mu.Lock()
	j.calls = append(j.calls, req)
	j.mu.Unlock()
	if j.join != nil {
		return j.join(req)
	}
	return txn.Status{ID: "0000000000000001", State: txn.Pending}, nil
}

type fixedAssociation struct {
	a   wifi.Association
	err error
}

func (f fixedAssociation) AssociatedUplink(context.Context) (wifi.Association, error) {
	return f.a, f.err
}

// Scan fixtures from internal/device/mock/testdata (HomeUplink, Neighbor2).
var (
	home     = wifi.JoinRequest{SSID: "HomeUplink", Security: wifi.SecurityWPA2, Key: secret}
	neighbor = wifi.JoinRequest{SSID: "Neighbor2", Security: wifi.SecurityWPA2, Key: "neighbor-key-5678"}
)

func newService(t *testing.T) (*Service, string, *recordingJoiner) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wifi-profiles.json")
	joiner := &recordingJoiner{}
	return New(path, joiner, fixedAssociation{err: wifi.ErrUnavailable}), path, joiner
}

func mustCreate(t *testing.T, s *Service, req wifi.JoinRequest) Summary {
	t.Helper()
	summary, err := s.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func ids(l List) []string {
	out := []string{}
	for _, p := range l.Profiles {
		out = append(out, p.ID)
	}
	return out
}

func TestAbsentStoreIsEmpty(t *testing.T) {
	s, path, _ := newService(t)
	list, err := s.List(context.Background())
	if err != nil || list.Profiles == nil || len(list.Profiles) != 0 || list.ConnectedID != nil {
		t.Fatal(list, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a read must not create the store", err)
	}
}

func TestRestartPersistsOrderAndMode(t *testing.T) {
	ctx := context.Background()
	s, path, _ := newService(t)
	a := mustCreate(t, s, home)
	b := mustCreate(t, s, wifi.JoinRequest{SSID: "Neighbor2", BSSID: "02:AB:00:00:00:01", Security: wifi.SecurityWPA2, Key: "neighbor-key-5678"})
	if !idPattern.MatchString(a.ID) || a.ID == b.ID || b.BSSID != "02:ab:00:00:00:01" {
		t.Fatal(a, b)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal(info, err)
	}
	restarted := New(path, nil, nil)
	list, err := restarted.List(ctx)
	if err != nil || !slices.Equal(ids(list), []string{a.ID, b.ID}) {
		t.Fatal(list, err)
	}
	req, err := restarted.snapshot(a.ID)
	if err != nil || req != home {
		t.Fatal("credentials not persisted", err)
	}
}

func TestCreateValidation(t *testing.T) {
	s, path, _ := newService(t)
	ctx := context.Background()
	var v wifi.ValidationError
	if _, err := s.Create(ctx, wifi.JoinRequest{SSID: "Test", Security: wifi.SecurityWPA2, Key: "short"}); !errors.As(err, &v) || v.Code != "wifi_key_invalid" {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, wifi.JoinRequest{SSID: "Test", Security: wifi.SecurityOther}); !errors.Is(err, wifi.ErrSecurityUnsupported) {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rejected input was stored")
	}
	mustCreate(t, s, home)
	if _, err := s.Create(ctx, wifi.JoinRequest{SSID: "HomeUplink", Security: wifi.SecurityOpen}); !errors.Is(err, ErrExists) {
		t.Fatal("duplicate selector", err)
	}
	// A pinned BSSID is a different selector from the unpinned SSID.
	pinned := mustCreate(t, s, wifi.JoinRequest{SSID: "HomeUplink", BSSID: "94:83:C4:1F:28:3A", Security: wifi.SecurityWPA2, Key: secret})
	if _, err := s.Create(ctx, wifi.JoinRequest{SSID: "HomeUplink", BSSID: pinned.BSSID, Security: wifi.SecurityOpen}); !errors.Is(err, ErrExists) {
		t.Fatal("duplicate normalized BSSID", err)
	}
}

func TestCreateLimit(t *testing.T) {
	s, _, _ := newService(t)
	for i := range maxProfiles {
		mustCreate(t, s, wifi.JoinRequest{SSID: "net" + string(rune('A'+i)), Security: wifi.SecurityOpen})
	}
	if _, err := s.Create(context.Background(), neighbor); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestIDCollisionExhaustion(t *testing.T) {
	s, path, _ := newService(t)
	s.random = bytes.NewReader(make([]byte, 64))
	first := mustCreate(t, s, home)
	before, _ := os.ReadFile(path)
	s.random = bytes.NewReader(make([]byte, 16*idDraws))
	if _, err := s.Create(context.Background(), neighbor); !errors.Is(err, ErrUnavailable) {
		t.Fatal("collisions must exhaust", err)
	}
	s.random = bytes.NewReader(nil)
	if _, err := s.Create(context.Background(), neighbor); !errors.Is(err, ErrUnavailable) {
		t.Fatal("entropy failure", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || first.ID != strings.Repeat("0", 32) {
		t.Fatal("store changed without a fresh ID")
	}
}

func TestReorderExact(t *testing.T) {
	ctx := context.Background()
	s, _, joiner := newService(t)
	a, b := mustCreate(t, s, home), mustCreate(t, s, neighbor)
	unknown := strings.Repeat("f", 32)
	for name, order := range map[string][]string{
		"omitted": {a.ID}, "extra": {a.ID, b.ID, unknown}, "duplicate": {a.ID, a.ID},
		"unknown": {a.ID, unknown}, "empty": {},
	} {
		if _, err := s.Reorder(ctx, order); !errors.Is(err, ErrOrderInvalid) {
			t.Fatal(name, err)
		}
	}
	list, err := s.Reorder(ctx, []string{b.ID, a.ID})
	if err != nil || !slices.Equal(ids(list), []string{b.ID, a.ID}) {
		t.Fatal(list, err)
	}
	if list, _ = s.List(ctx); !slices.Equal(ids(list), []string{b.ID, a.ID}) {
		t.Fatal("order not persisted")
	}
	if len(joiner.calls) != 0 {
		t.Fatal("reorder touched the live STA")
	}
}

func TestDeleteLeavesLiveStateAlone(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "wifi-profiles.json")
	joiner := &recordingJoiner{}
	s := New(path, joiner, fixedAssociation{a: wifi.Association{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}})
	a, b := mustCreate(t, s, home), mustCreate(t, s, neighbor)
	list, err := s.List(ctx)
	if err != nil || list.ConnectedID == nil || *list.ConnectedID != a.ID {
		t.Fatal(list, err)
	}
	list, err = s.Delete(ctx, a.ID)
	if err != nil || !slices.Equal(ids(list), []string{b.ID}) || list.ConnectedID != nil {
		t.Fatal(list, err)
	}
	if _, err := s.Delete(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if len(joiner.calls) != 0 {
		t.Fatal("save or forget touched the live STA")
	}
}

func TestMalformedStorePreserved(t *testing.T) {
	ctx := context.Background()
	valid := `{"id":"` + strings.Repeat("a", 32) + `","ssid":"HomeUplink","security":"wpa2","key":"` + secret + `"}`
	for name, content := range map[string]string{
		"empty":              "",
		"null":               "null",
		"malformed":          "{",
		"missing profiles":   "{}",
		"null profiles":      `{"profiles":null}`,
		"bad id":             `{"profiles":[{"id":"x","ssid":"A","security":"open"}]}`,
		"unsupported":        `{"profiles":[{"id":"` + strings.Repeat("b", 32) + `","ssid":"A","security":"other"}]}`,
		"short key":          `{"profiles":[{"id":"` + strings.Repeat("b", 32) + `","ssid":"A","security":"wpa2","key":"short"}]}`,
		"upper BSSID":        `{"profiles":[{"id":"` + strings.Repeat("b", 32) + `","ssid":"A","bssid":"02:AB:00:00:00:01","security":"open"}]}`,
		"duplicate id":       `{"profiles":[` + valid + `,` + strings.Replace(valid, "HomeUplink", "Other", 1) + `]}`,
		"duplicate selector": `{"profiles":[` + valid + `,` + strings.Replace(valid, strings.Repeat("a", 32), strings.Repeat("b", 32), 1) + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			s, path, _ := newService(t)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.List(ctx); !errors.Is(err, ErrUnavailable) {
				t.Fatal(err)
			}
			if _, err := s.Create(ctx, neighbor); !errors.Is(err, ErrUnavailable) {
				t.Fatal(err)
			}
			if _, err := s.Delete(ctx, strings.Repeat("a", 32)); !errors.Is(err, ErrUnavailable) {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); string(got) != content {
				t.Fatal("invalid store was rewritten")
			}
		})
	}
	t.Run("over cap", func(t *testing.T) {
		s, path, _ := newService(t)
		var d stored
		for i := range maxProfiles + 1 {
			d.Profiles = append(d.Profiles, profile{ID: strings.Repeat("0", 30) + string("0123456789abcdef"[i/16]) + string("0123456789abcdef"[i%16]), SSID: "n" + string(rune('A'+i)), Security: wifi.SecurityOpen})
		}
		b, _ := json.Marshal(d)
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.List(ctx); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	})
}

func TestFailedWritePreservesStore(t *testing.T) {
	ctx := context.Background()
	s, path, _ := newService(t)
	a, b := mustCreate(t, s, home), mustCreate(t, s, neighbor)
	before, _ := os.ReadFile(path)
	s.writeJSON = func(string, any, os.FileMode) error { return errors.New("disk full") }
	if _, err := s.Create(ctx, wifi.JoinRequest{SSID: "Third", Security: wifi.SecurityOpen}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.Reorder(ctx, []string{b.ID, a.ID}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.Delete(ctx, a.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("failed write changed the store")
	}
}

func TestConnectedPrecedence(t *testing.T) {
	unpinned := Summary{ID: "u", SSID: "HomeUplink"}
	pinned := Summary{ID: "p", SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}
	other := Summary{ID: "o", SSID: "HomeUplink", BSSID: "02:00:00:00:00:01"}
	observed := wifi.Association{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}
	for _, tc := range []struct {
		name     string
		profiles []Summary
		a        wifi.Association
		want     string
	}{
		{"pinned wins", []Summary{unpinned, pinned}, observed, "p"},
		{"unpinned by SSID", []Summary{other, unpinned}, observed, "u"},
		{"other BSSID only", []Summary{other}, observed, ""},
		{"disconnected keeps BSSID", []Summary{unpinned, pinned}, wifi.Association{BSSID: observed.BSSID}, ""},
	} {
		got := Connected(tc.profiles, tc.a)
		if (got == nil && tc.want != "") || (got != nil && *got != tc.want) {
			t.Fatal(tc.name, got)
		}
	}
}

func TestSelectSnapshotAndUnchangedStore(t *testing.T) {
	ctx := context.Background()
	s, path, joiner := newService(t)
	a := mustCreate(t, s, home)
	before, _ := os.ReadFile(path)
	release := make(chan struct{})
	joined := make(chan struct{})
	joiner.join = func(wifi.JoinRequest) (txn.Status, error) {
		close(joined)
		<-release
		return txn.Status{ID: "0000000000000001", State: txn.RolledBack}, txn.ErrProbeFailed
	}
	type result struct {
		st  txn.Status
		err error
	}
	done := make(chan result, 1)
	go func() {
		st, err := s.Select(ctx, a.ID)
		done <- result{st, err}
	}()
	<-joined
	// The store lock is released during Join; a delete does not cancel the copy.
	if _, err := s.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	r := <-done
	if !errors.Is(r.err, txn.ErrProbeFailed) || r.st.State != txn.RolledBack || len(joiner.calls) != 1 || joiner.calls[0] != home {
		t.Fatal(r, joiner.calls)
	}
	if _, err := s.Select(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// Selecting another profile leaves stored credentials and order untouched.
	b := mustCreate(t, s, home)
	joiner.join = nil
	before, _ = os.ReadFile(path)
	if st, err := s.Select(ctx, b.ID); err != nil || st.State != txn.Pending {
		t.Fatal(st, err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("select changed the store")
	}
	if _, err := New(path, nil, nil).Select(ctx, b.ID); !errors.Is(err, wifi.ErrUnavailable) {
		t.Fatal("nil joiner", err)
	}
}

func TestPublicShapesHaveNoKey(t *testing.T) {
	s, _, _ := newService(t)
	summary := mustCreate(t, s, home)
	list, _ := s.List(context.Background())
	for _, v := range []any{summary, list} {
		b, _ := json.Marshal(v)
		if bytes.Contains(b, []byte(secret)) || bytes.Contains(b, []byte(`"key"`)) {
			t.Fatalf("public shape leaks credentials: %s", b)
		}
	}
}
