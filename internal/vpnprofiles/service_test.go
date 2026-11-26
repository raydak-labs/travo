package vpnprofiles

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func key(b byte) string {
	var data [32]byte
	data[0] = b
	return base64.StdEncoding.EncodeToString(data[:])
}
func input() string {
	return "[Interface]\nPrivateKey=" + key(1) + "\nAddress=10.0.0.2/24, fd00::2/64\nDNS=1.1.1.1, ::1\nMTU=1420\nListenPort=51820\n[Peer]\nPublicKey=" + key(2) + "\nAllowedIPs=10.0.0.5/24, fd00::42/64\nEndpoint=[2001:db8::1]:51820\nPersistentKeepalive=0\n"
}
func TestParse(t *testing.T) {
	raw := strings.ReplaceAll("# comment\n"+strings.Replace(input(), "[Interface]", "[iNTERFACE]", 1)+"[Peer]\nPublicKey="+key(3)+"\nPresharedKey="+key(4)+"\nAllowedIPs=0.0.0.0/0\nEndpoint=example.test:51820\n", "\n", "\r\n")
	c, err := Parse(raw)
	if err != nil || len(c.Peers) != 2 || c.Addresses[0].String() != "10.0.0.2/24" || c.Addresses[1].String() != "fd00::2/64" || c.Peers[0].AllowedIPs[0].String() != "10.0.0.0/24" || c.Peers[0].AllowedIPs[1].String() != "fd00::/64" {
		t.Fatal("unexpected parsed profile", err)
	}
	for name, raw := range map[string]string{
		"hook":           input() + "PostUp=secret-command",
		"duplicate":      input() + "AllowedIPs=::/0",
		"duplicate peer": input() + "[Peer]\nPublicKey=" + key(2) + "\nAllowedIPs=::/0",
		"zero":           strings.Replace(input(), key(1), key(0), 1),
		"invalid key":    strings.Replace(input(), key(1), "secret-value", 1),
		"unbracketed":    strings.Replace(input(), "[2001:db8::1]", "2001:db8::1", 1),
		"no address":     strings.Replace(input(), "Address=10.0.0.2/24, fd00::2/64\n", "", 1),
		"size":           input() + strings.Repeat(" ", maxConfig),
		"peers":          input() + strings.Repeat("[Peer]\nPublicKey="+key(3)+"\nAllowedIPs=::/0\n", maxPeers),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(raw); err != ErrInvalid {
				t.Fatal("expected generic error", err)
			}
		})
	}
}
func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s := New(path)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, err := s.Import("same name", input()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	s = New(path)
	list, err := s.List()
	if err != nil || len(list.Profiles) != 12 || list.SelectedID != nil {
		t.Fatal(list, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal(info, err)
	}
	id := list.Profiles[0].ID
	list, err = s.Select(id)
	if err != nil || list.SelectedID == nil || *list.SelectedID != id {
		t.Fatal(list, err)
	}
	list, err = s.Delete(id)
	if err != nil || list.SelectedID != nil || len(list.Profiles) != 11 {
		t.Fatal(list, err)
	}
	if _, err = s.Select(id); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestCorruptStateRefusesWrites(t *testing.T) {
	for _, raw := range []string{`null`, `{"profiles":null}`, `{"profiles":[],"selected_id":"not-present"}`, `{"profiles":[{"id":"00000000000000000000000000000000","name":"valid","config":{}}]}`} {
		path := filepath.Join(t.TempDir(), "profiles.json")
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		s := New(path)
		if _, err := s.Import("valid", input()); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != raw {
			t.Fatal("corrupt state was replaced")
		}
	}
}
func TestWriteFailure(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "missing", "profiles.json"))
	if _, err := s.Import("valid", input()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list.Profiles) != 0 {
		t.Fatal(list, err)
	}
}
