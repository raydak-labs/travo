package vpnprofiles

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

const maxProfiles = 32

var (
	ErrUnavailable = errors.New("VPN profiles unavailable")
	ErrNotFound    = errors.New("VPN profile not found")
	ErrLimit       = errors.New("VPN profile limit reached")
	idPattern      = regexp.MustCompile("^[0-9a-f]{32}$")
)

type Summary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type List struct {
	Profiles   []Summary `json:"profiles" nullable:"false"`
	SelectedID *string   `json:"selected_id" nullable:"true"`
}
type profile struct {
	Summary
	Config Config `json:"config"`
}
type stored struct {
	Profiles   []profile `json:"profiles"`
	SelectedID *string   `json:"selected_id"`
}
type Service struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Service { return &Service{path: path} }

func validName(name string) bool {
	return name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= 64 &&
		strings.TrimSpace(name) == name && strings.IndexFunc(name, unicode.IsControl) == -1
}
func (s *Service) load() (stored, error) {
	var data stored
	err := atomicfile.ReadJSON(s.path, &data)
	if errors.Is(err, os.ErrNotExist) {
		return stored{Profiles: []profile{}}, nil
	}
	if err != nil || data.Profiles == nil || len(data.Profiles) > maxProfiles {
		return stored{}, ErrUnavailable
	}
	seen := make(map[string]bool, len(data.Profiles))
	for _, p := range data.Profiles {
		if seen[p.ID] || !idPattern.MatchString(p.ID) || !validName(p.Name) || !p.Config.valid() {
			return stored{}, ErrUnavailable
		}
		seen[p.ID] = true
	}
	if data.SelectedID != nil && !seen[*data.SelectedID] {
		return stored{}, ErrUnavailable
	}
	return data, nil
}
func (d stored) list() List {
	out := List{Profiles: make([]Summary, 0, len(d.Profiles)), SelectedID: d.SelectedID}
	for _, p := range d.Profiles {
		out.Profiles = append(out.Profiles, p.Summary)
	}
	return out
}
func (s *Service) List() (List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	return d.list(), err
}
func (s *Service) write(d stored) error {
	if atomicfile.WriteJSON(s.path, d, 0600) != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) Import(name, raw string) (Summary, error) {
	if strings.IndexFunc(name, unicode.IsControl) != -1 {
		return Summary{}, ErrInvalid
	}
	name = strings.TrimSpace(name)
	if !validName(name) {
		return Summary{}, ErrInvalid
	}
	config, err := Parse(raw)
	if err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Summary{}, err
	}
	if len(d.Profiles) >= maxProfiles {
		return Summary{}, ErrLimit
	}
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Summary{}, ErrUnavailable
	}
	result := Summary{ID: hex.EncodeToString(entropy[:]), Name: name}
	if d.index(result.ID) >= 0 {
		return Summary{}, ErrUnavailable
	}
	d.Profiles = append(d.Profiles, profile{Summary: result, Config: config})
	if err := s.write(d); err != nil {
		return Summary{}, err
	}
	return result, nil
}
func (d stored) index(id string) int {
	for i, p := range d.Profiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}
func (s *Service) Select(id string) (List, error) { return s.change(id, false) }
func (s *Service) Delete(id string) (List, error) { return s.change(id, true) }
func (s *Service) change(id string, del bool) (List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return List{}, err
	}
	i := d.index(id)
	if i < 0 {
		return List{}, ErrNotFound
	}
	if del {
		d.Profiles = append(d.Profiles[:i], d.Profiles[i+1:]...)
		if d.SelectedID != nil && *d.SelectedID == id {
			d.SelectedID = nil
		}
	} else {
		if d.SelectedID != nil && *d.SelectedID == id {
			return d.list(), nil
		}
		d.SelectedID = &id
	}
	if err := s.write(d); err != nil {
		return List{}, err
	}
	return d.list(), nil
}

// SelectedConfig returns a private snapshot for applying the selected profile.
func (s *Service) SelectedConfig() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return Config{}, err
	}
	if data.SelectedID == nil {
		return Config{}, ErrNotFound
	}
	return data.Profiles[data.index(*data.SelectedID)].Config, nil
}
