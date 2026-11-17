package dnslayer

import (
	"context"
	"fmt"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// Adopt turns live dnsmasq options that already equal replace layer l (written before the
// stack existed, e.g. old Travo's AdGuard forwarding as the base) into an active layer over
// a base without l's server/noresolv. Popping l then really removes it. It returns nil when
// a record exists or the live state is anything other than exactly l: nothing to adopt.
func Adopt(rec *Record, cur State, l Layer) (*Record, error) {
	if err := validate(l); err != nil {
		return nil, err
	}
	if additive[l.Name] || rec != nil {
		return nil, nil
	}
	noresolv := "0"
	if l.NoResolv {
		noresolv = "1"
	}
	if !cur.Server.Set || !slices.Equal(cur.Server.Values, l.Servers) || !cur.NoResolv.Set || cur.NoResolv.Value != noresolv {
		return nil, nil
	}
	base := State{RebindProtection: cur.RebindProtection}
	out := Record{Version: recordVersion, Base: base, Layers: []Layer{l}}
	return &out, nil
}

// Adopt records an exact live match of l as the active layer. It writes no DNS option, so
// it needs no guard: the record only describes what is already live.
func (s *Service) Adopt(ctx context.Context, l Layer) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pending, err := s.guardActive(); err != nil || pending {
		return false, err // an unfinished change owns the record; Resume first
	}
	rec, err := s.Load()
	if err != nil {
		return false, err
	}
	_, cur, err := ReadState(ctx, s.Device)
	if err != nil {
		return false, err
	}
	next, err := Adopt(rec, cur, l)
	if err != nil || next == nil {
		return false, err
	}
	if err := atomicfile.WriteJSON(s.RecordPath, next, 0o600); err != nil {
		return false, fmt.Errorf("dns layers: save adopted record: %w", err)
	}
	return true, nil
}
