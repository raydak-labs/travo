package vpntunnel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

type enableProof struct {
	started    int64
	wasEnabled bool
	previous   map[string]time.Time
	configured map[string]struct{}
}

func (s *Service) proveEnabled(ctx context.Context, proof enableProof) error {
	if proof.wasEnabled {
		if err := s.control.SetInterfaceUp(ctx, iface, false); err != nil {
			return errProof
		}
		if err := s.waitLinkAbsent(ctx); err != nil {
			return err
		}
		if err := s.control.SetInterfaceUp(ctx, iface, true); err != nil {
			return errProof
		}
	} else if err := s.control.ReloadNetwork(ctx); err != nil {
		return errProof
	}
	return poll(ctx, func(ctx context.Context) (bool, error) {
		raw, err := s.ubus.Call(ctx, "network.interface."+iface, "status", nil)
		var status struct {
			Up bool `json:"up"`
		}
		if err != nil || json.Unmarshal(raw, &status) != nil || !status.Up {
			return false, nil
		}
		peers, err := s.handshakes.LatestHandshakes(ctx, iface)
		if err != nil {
			return false, nil
		}
		now := s.now().Unix()
		for _, peer := range peers {
			_, configured := proof.configured[peer.PublicKey]
			if configured && !peer.At.IsZero() && peer.At.Unix() >= proof.started &&
				peer.At.Unix() <= now && peer.At.After(proof.previous[peer.PublicKey]) {
				return true, nil
			}
		}
		return false, nil
	})
}

func (s *Service) proveDisabled(ctx context.Context) error {
	if err := s.control.ReloadNetwork(ctx); err != nil {
		return errProof
	}
	if err := s.waitLinkAbsent(ctx); err != nil {
		return err
	}
	ok, err := s.hasUplinkDefault(ctx)
	if err != nil {
		return errProof
	}
	if ok {
		return nil
	}
	logical, err := s.recoveryInterface(ctx)
	if err != nil {
		return err
	}
	if err := s.control.SetInterfaceUp(ctx, logical, false); err != nil {
		return errProof
	}
	if err := s.control.SetInterfaceUp(ctx, logical, true); err != nil {
		return errProof
	}
	return poll(ctx, s.hasUplinkDefault)
}

func (s *Service) hasUplinkDefault(ctx context.Context) (bool, error) {
	route, found, err := s.routes.DefaultIPv4Route(ctx)
	if err != nil {
		return false, errProof
	}
	return found && route.Device != "" && route.Device != iface, nil
}

func (s *Service) recoveryInterface(ctx context.Context) (string, error) {
	raw, err := s.ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return "", errProof
	}
	interfaces, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return "", errProof
	}
	logical := ""
	for _, entry := range interfaces {
		if entry.Name == iface || !entry.Up || entry.L3Device == "" || entry.L3Device == iface ||
			len(entry.IPv4) == 0 || !entry.HasDefRte {
			continue
		}
		if logical != "" {
			return "", errProof
		}
		logical = entry.Name
	}
	if logical == "" {
		return "", errProof
	}
	return logical, nil
}

func (s *Service) waitLinkAbsent(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return poll(ctx, func(ctx context.Context) (bool, error) {
		present, err := s.control.WireGuardLinkPresent(ctx)
		if err != nil {
			return false, errProof
		}
		return !present, nil
	})
}

func poll(ctx context.Context, check func(context.Context) (bool, error)) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		ok, err := check(ctx)
		if err != nil {
			return err
		}
		if ok && ctx.Err() == nil {
			return nil
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
			continue
		}
	}
	return errors.Join(errProof, ctx.Err())
}
