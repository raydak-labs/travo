package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

const vpnImportOperation = "postVPNProfile"

type vpnImportInput struct {
	Body struct {
		Name   string `json:"name"`
		Config string `json:"config"`
	}
}
type vpnProfileID struct {
	ID string `path:"id" pattern:"^[0-9a-f]{32}$"`
}
type vpnListOutput struct{ Body vpnprofiles.List }
type vpnImportOutput struct{ Body vpnprofiles.Summary }

func (s *server) registerVPNProfiles(a huma.API) {
	const path = "/api/v1/vpn/profiles"
	register(s, false, op("getVPNProfiles", http.MethodGet, path, "List saved WireGuard profiles", "vpn", 503), a, s.listVPNProfiles)
	importOp := op(vpnImportOperation, http.MethodPost, path, "Import a WireGuard profile", "vpn", 400, 409, 413, 503)
	importOp.DefaultStatus = http.StatusCreated
	importOp.MaxBodyBytes = 512 << 10
	register(s, false, importOp, a, s.importVPNProfile)
	register(s, false, op("selectVPNProfile", http.MethodPost, path+"/{id}/select", "Select a WireGuard profile", "vpn", 400, 404, 503), a, s.changeVPNProfile(false))
	register(s, false, op("deleteVPNProfile", http.MethodDelete, path+"/{id}", "Delete a WireGuard profile", "vpn", 400, 404, 503), a, s.changeVPNProfile(true))
}
func vpnError(err error) error {
	switch {
	case errors.Is(err, vpnprofiles.ErrInvalid):
		return fail(400, "invalid_profile", "Invalid WireGuard profile")
	case errors.Is(err, vpnprofiles.ErrNotFound):
		return fail(404, "profile_not_found", "WireGuard profile not found")
	case errors.Is(err, vpnprofiles.ErrLimit):
		return fail(409, "profile_limit", "WireGuard profile limit reached")
	default:
		return fail(503, "profiles_unavailable", "WireGuard profiles are unavailable")
	}
}
func (s *server) listVPNProfiles(context.Context, *struct{}) (*vpnListOutput, error) {
	if s.VPNProfiles == nil {
		return nil, vpnError(vpnprofiles.ErrUnavailable)
	}
	list, err := s.VPNProfiles.List()
	if err != nil {
		return nil, vpnError(err)
	}
	return &vpnListOutput{Body: list}, nil
}
func (s *server) importVPNProfile(_ context.Context, in *vpnImportInput) (*vpnImportOutput, error) {
	if s.VPNProfiles == nil {
		return nil, vpnError(vpnprofiles.ErrUnavailable)
	}
	summary, err := s.VPNProfiles.Import(in.Body.Name, in.Body.Config)
	if err != nil {
		return nil, vpnError(err)
	}
	return &vpnImportOutput{Body: summary}, nil
}
func (s *server) changeVPNProfile(del bool) func(context.Context, *vpnProfileID) (*vpnListOutput, error) {
	return func(_ context.Context, in *vpnProfileID) (*vpnListOutput, error) {
		if s.VPNProfiles == nil {
			return nil, vpnError(vpnprofiles.ErrUnavailable)
		}
		change := s.VPNProfiles.Select
		if del {
			change = s.VPNProfiles.Delete
		}
		list, err := change(in.ID)
		if err != nil {
			return nil, vpnError(err)
		}
		return &vpnListOutput{Body: list}, nil
	}
}
