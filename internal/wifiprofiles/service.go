// Package wifiprofiles stores ordered saved uplink credentials. The JSON store owns
// preferences only; live wireless configuration stays in UCI and changes solely
// through the guarded WiFi join (docs/adr/0009).
package wifiprofiles

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"regexp"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

const (
	maxProfiles = 32
	idDraws     = 3
)

var (
	ErrUnavailable  = errors.New("WiFi profiles unavailable")
	ErrNotFound     = errors.New("WiFi profile not found")
	ErrExists       = errors.New("WiFi profile already saved")
	ErrLimit        = errors.New("WiFi profile limit reached")
	ErrOrderInvalid = errors.New("WiFi profile order invalid")
	idPattern       = regexp.MustCompile("^[0-9a-f]{32}$")
)

// Summary is the public, keyless view of a saved uplink.
type Summary struct {
	ID       string        `json:"id"`
	SSID     string        `json:"ssid"`
	BSSID    string        `json:"bssid,omitempty"`
	Security wifi.Security `json:"security" enum:"open,wpa2,wpa2_wpa3"`
}

// List is in priority order; ConnectedID is null unless association is proven.
type List struct {
	Profiles    []Summary `json:"profiles" nullable:"false"`
	ConnectedID *string   `json:"connected_id" nullable:"true"`
}

type Joiner interface {
	Join(context.Context, wifi.JoinRequest) (txn.Status, error)
}

type AssociationReader interface {
	AssociatedUplink(context.Context) (wifi.Association, error)
}

// profile is the private persisted shape; it is the only place the key lives.
type profile struct {
	ID       string        `json:"id"`
	SSID     string        `json:"ssid"`
	BSSID    string        `json:"bssid,omitempty"`
	Security wifi.Security `json:"security"`
	Key      string        `json:"key,omitempty"`
}

type stored struct {
	Profiles []profile `json:"profiles"`
}

type Service struct {
	path        string
	joiner      Joiner
	association AssociationReader
	random      io.Reader
	writeJSON   func(string, any, os.FileMode) error
	mu          sync.Mutex
}

func New(path string, joiner Joiner, association AssociationReader) *Service {
	return &Service{path: path, joiner: joiner, association: association, random: rand.Reader, writeJSON: atomicfile.WriteJSON}
}

func (p profile) request() wifi.JoinRequest {
	return wifi.JoinRequest{SSID: p.SSID, BSSID: p.BSSID, Key: p.Key, Security: p.Security}
}

func (p profile) summary() Summary {
	return Summary{ID: p.ID, SSID: p.SSID, BSSID: p.BSSID, Security: p.Security}
}

type selector struct{ ssid, bssid string }

func (p profile) selector() selector { return selector{p.SSID, p.BSSID} }

// load fails closed: anything but an absent file or a fully valid store is
// ErrUnavailable, and the file is never rewritten from a failed read.
func (s *Service) load() (stored, error) {
	var data stored
	err := atomicfile.ReadJSON(s.path, &data)
	if errors.Is(err, os.ErrNotExist) {
		return stored{Profiles: []profile{}}, nil
	}
	if err != nil || data.Profiles == nil || len(data.Profiles) > maxProfiles {
		return stored{}, ErrUnavailable
	}
	ids := make(map[string]bool, len(data.Profiles))
	selectors := make(map[selector]bool, len(data.Profiles))
	for _, p := range data.Profiles {
		normalized, err := wifi.ValidateJoinRequest(p.request())
		if err != nil || normalized.BSSID != p.BSSID || !idPattern.MatchString(p.ID) || ids[p.ID] || selectors[p.selector()] {
			return stored{}, ErrUnavailable
		}
		ids[p.ID], selectors[p.selector()] = true, true
	}
	return data, nil
}

func (s *Service) write(d stored) error {
	if s.writeJSON(s.path, d, 0o600) != nil {
		return ErrUnavailable
	}
	return nil
}

func (d stored) index(id string) int {
	for i, p := range d.Profiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func (s *Service) newID(d stored) (string, error) {
	for range idDraws {
		var entropy [16]byte
		if _, err := io.ReadFull(s.random, entropy[:]); err != nil {
			return "", ErrUnavailable
		}
		if id := hex.EncodeToString(entropy[:]); d.index(id) < 0 {
			return id, nil
		}
	}
	return "", ErrUnavailable
}

// List returns saved uplinks in priority order with the proven connected one.
func (s *Service) List(ctx context.Context) (List, error) {
	s.mu.Lock()
	d, err := s.load()
	s.mu.Unlock()
	if err != nil {
		return List{}, err
	}
	return s.list(ctx, d), nil
}

// list reads association after the store lock is released.
func (s *Service) list(ctx context.Context, d stored) List {
	out := List{Profiles: make([]Summary, 0, len(d.Profiles))}
	for _, p := range d.Profiles {
		out.Profiles = append(out.Profiles, p.summary())
	}
	if s.association == nil {
		return out
	}
	if a, err := s.association.AssociatedUplink(ctx); err == nil {
		out.ConnectedID = Connected(out.Profiles, a)
	}
	return out
}

// Connected names the profile matching an observed association. An exact pinned
// match wins over an unpinned SSID match; no SSID means no association.
func Connected(profiles []Summary, a wifi.Association) *string {
	if a.SSID == "" {
		return nil
	}
	var unpinned *string
	for _, p := range profiles {
		if p.SSID != a.SSID {
			continue
		}
		if p.BSSID == a.BSSID {
			return &p.ID
		}
		if p.BSSID == "" && unpinned == nil {
			unpinned = &p.ID
		}
	}
	return unpinned
}

// Create saves credentials at the lowest priority. It never touches the live STA.
func (s *Service) Create(_ context.Context, req wifi.JoinRequest) (Summary, error) {
	req, err := wifi.ValidateJoinRequest(req)
	if err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Summary{}, err
	}
	p := profile{SSID: req.SSID, BSSID: req.BSSID, Security: req.Security, Key: req.Key}
	for _, existing := range d.Profiles {
		if existing.selector() == p.selector() {
			return Summary{}, ErrExists
		}
	}
	if len(d.Profiles) >= maxProfiles {
		return Summary{}, ErrLimit
	}
	if p.ID, err = s.newID(d); err != nil {
		return Summary{}, err
	}
	d.Profiles = append(d.Profiles, p)
	if err := s.write(d); err != nil {
		return Summary{}, err
	}
	return p.summary(), nil
}

// Reorder requires an exact permutation of the saved IDs.
func (s *Service) Reorder(ctx context.Context, ids []string) (List, error) {
	s.mu.Lock()
	d, err := s.reorder(ids)
	s.mu.Unlock()
	if err != nil {
		return List{}, err
	}
	return s.list(ctx, d), nil
}

func (s *Service) reorder(ids []string) (stored, error) {
	d, err := s.load()
	if err != nil {
		return stored{}, err
	}
	if len(ids) != len(d.Profiles) {
		return stored{}, ErrOrderInvalid
	}
	ordered := make([]profile, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		i := d.index(id)
		if i < 0 || seen[id] {
			return stored{}, ErrOrderInvalid
		}
		seen[id] = true
		ordered = append(ordered, d.Profiles[i])
	}
	d.Profiles = ordered
	return d, s.write(d)
}

// Delete forgets a profile. A connected STA stays up: no live state changes.
func (s *Service) Delete(ctx context.Context, id string) (List, error) {
	s.mu.Lock()
	d, err := s.delete(id)
	s.mu.Unlock()
	if err != nil {
		return List{}, err
	}
	return s.list(ctx, d), nil
}

func (s *Service) delete(id string) (stored, error) {
	d, err := s.load()
	if err != nil {
		return stored{}, err
	}
	i := d.index(id)
	if i < 0 {
		return stored{}, ErrNotFound
	}
	d.Profiles = append(d.Profiles[:i], d.Profiles[i+1:]...)
	return d, s.write(d)
}

// Select joins a copy of the saved credentials through the guarded join. The
// store is unchanged whatever the transaction outcome.
func (s *Service) Select(ctx context.Context, id string) (txn.Status, error) {
	s.mu.Lock()
	req, err := s.snapshot(id)
	s.mu.Unlock()
	if err != nil {
		return txn.Status{}, err
	}
	if s.joiner == nil {
		return txn.Status{}, wifi.ErrUnavailable
	}
	return s.joiner.Join(ctx, req)
}

func (s *Service) snapshot(id string) (wifi.JoinRequest, error) {
	d, err := s.load()
	if err != nil {
		return wifi.JoinRequest{}, err
	}
	i := d.index(id)
	if i < 0 {
		return wifi.JoinRequest{}, ErrNotFound
	}
	return d.Profiles[i].request(), nil
}
