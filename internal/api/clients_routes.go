package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/clients"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Clients lists LAN clients and changes their alias, reservation and access (internal/clients).
type Clients interface {
	List(context.Context) (clients.Listing, error)
	SetAlias(mac, alias string) error
	ListReservations(context.Context) ([]clients.Reservation, error)
	Reserve(context.Context, clients.Reservation) (clients.Reservation, error)
	Unreserve(ctx context.Context, mac string) error
	SetBlocked(ctx context.Context, mac string, blocked bool, caller netip.Addr) (txn.Status, error)
	Kick(ctx context.Context, mac string, caller netip.Addr) error
}

type clientsOutput struct {
	Body struct {
		Clients []clients.Client `json:"clients" nullable:"false"`
		Partial bool             `json:"partial" doc:"A source (leases, neighbours, AP stations, firewall) could not be read; rows may lack details"`
	}
}

type macPath struct {
	MAC string `path:"mac" pattern:"^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$" doc:"Client MAC"`
}

type aliasInput struct {
	MAC  string `path:"mac" pattern:"^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$" doc:"Client MAC"`
	Body struct {
		Alias string `json:"alias" maxLength:"256" doc:"Name for the client; empty clears it"`
	}
}

type reservationsOutput struct {
	Body struct {
		Reservations []clients.Reservation `json:"reservations" nullable:"false"`
	}
}

type reservationInput struct {
	Body struct {
		MAC  string `json:"mac" maxLength:"17"`
		IP   string `json:"ip" maxLength:"15" doc:"IPv4 address inside the lan subnet"`
		Name string `json:"name,omitempty" maxLength:"63" doc:"Host name (one DNS label); empty for none"`
	}
}

type reservationOutput struct{ Body clients.Reservation }

type blockInput struct {
	MAC  string `path:"mac" pattern:"^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$" doc:"Client MAC"`
	Body struct {
		Blocked bool `json:"blocked" doc:"true rejects the client's forwarded traffic; false removes the rule"`
	}
}

func (s *server) registerClients(a huma.API) {
	register(s, false, op("listClients", http.MethodGet, "/api/v1/clients", "LAN clients from leases, neighbours and AP stations", "clients", 503), a, s.listClients)
	alias := op("setClientAlias", http.MethodPut, "/api/v1/clients/{mac}/alias", "Name a client (stored by Travo, not in UCI)", "clients", 400, 503)
	alias.MaxBodyBytes = 1024
	register(s, false, alias, a, s.setClientAlias)
	register(s, false, op("listReservations", http.MethodGet, "/api/v1/clients/reservations", "DHCP reservations (dhcp host sections)", "clients", 503), a, s.listReservations)
	reserve := op("reserveClient", http.MethodPost, "/api/v1/clients/reservations", "Add or change a DHCP reservation", "clients", 400, 409, 503)
	reserve.MaxBodyBytes = 1024
	register(s, false, reserve, a, s.reserveClient)
	register(s, false, op("deleteReservation", http.MethodDelete, "/api/v1/clients/reservations/{mac}", "Remove a DHCP reservation", "clients", 400, 404, 409, 503), a, s.deleteReservation)
	block := op("setClientBlocked", http.MethodPost, "/api/v1/clients/{mac}/block", "Block or unblock a client's internet access (guarded transaction)", "clients", 400, 409, 503)
	block.DefaultStatus = http.StatusAccepted
	block.MaxBodyBytes = 256
	register(s, false, block, a, s.setClientBlocked)
	register(s, false, op("kickClient", http.MethodPost, "/api/v1/clients/{mac}/kick", "Disconnect a WiFi client once (it may rejoin)", "clients", 400, 404, 409, 503), a, s.kickClient)
}

func clientsUnavailable() error {
	return fail(http.StatusServiceUnavailable, "clients_unavailable", "client data is unavailable")
}

// clientsFailure maps the expected failures; anything else is logged and 503.
func clientsFailure(err error) error {
	var fe *clients.FieldError
	switch {
	case errors.As(err, &fe):
		return fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, clients.ErrAliasInvalid):
		return fail(http.StatusBadRequest, "invalid_request", "alias: "+err.Error())
	case errors.Is(err, clients.ErrAliasesFull):
		return fail(http.StatusConflict, "client_aliases_full", "too many client aliases")
	case errors.Is(err, clients.ErrIsCaller):
		return fail(http.StatusConflict, "client_is_caller", "this is the device you are using")
	case errors.Is(err, clients.ErrNotAssociated):
		return fail(http.StatusNotFound, "client_not_associated", "the client is not connected to an access point")
	case errors.Is(err, clients.ErrReservationNotFound):
		return fail(http.StatusNotFound, "reservation_not_found", "no reservation for this client")
	case errors.Is(err, clients.ErrReservationShared):
		return fail(http.StatusConflict, "reservation_shared", "the reservation is shared with other devices; edit it in LuCI")
	case errors.Is(err, clients.ErrIPTaken):
		return fail(http.StatusConflict, "reservation_ip_taken", "the address is reserved for another device")
	case errors.Is(err, clients.ErrNoLAN):
		return fail(http.StatusConflict, "reservation_no_lan", "the lan IPv4 subnet is unknown")
	case errors.Is(err, clients.ErrBlockConflict):
		return fail(http.StatusConflict, "client_block_conflict", "a foreign firewall section uses the block rule's name")
	case errors.Is(err, clients.ErrTransactionPending):
		return fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return fail(http.StatusConflict, "client_no_change", "the client is already in that state")
	case errors.Is(err, txn.ErrForeignStaged):
		return fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("clients", "err", err)
	return clientsUnavailable()
}

func (s *server) listClients(ctx context.Context, _ *struct{}) (*clientsOutput, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	list, err := s.Clients.List(ctx)
	if err != nil {
		return nil, clientsFailure(err)
	}
	out := &clientsOutput{}
	out.Body.Clients, out.Body.Partial = list.Clients, list.Partial
	if out.Body.Clients == nil {
		out.Body.Clients = []clients.Client{}
	}
	return out, nil
}

func (s *server) setClientAlias(_ context.Context, in *aliasInput) (*struct{ Body okBody }, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	if err := s.Clients.SetAlias(in.MAC, in.Body.Alias); err != nil {
		return nil, clientsFailure(err)
	}
	return okResponse(), nil
}

func okResponse() *struct{ Body okBody } {
	out := &struct{ Body okBody }{}
	out.Body.Status = "ok"
	return out
}

func (s *server) listReservations(ctx context.Context, _ *struct{}) (*reservationsOutput, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	list, err := s.Clients.ListReservations(ctx)
	if err != nil {
		return nil, clientsFailure(err)
	}
	out := &reservationsOutput{}
	out.Body.Reservations = list
	if list == nil {
		out.Body.Reservations = []clients.Reservation{}
	}
	return out, nil
}

func (s *server) reserveClient(ctx context.Context, in *reservationInput) (*reservationOutput, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	r, err := s.Clients.Reserve(ctx, clients.Reservation{MAC: in.Body.MAC, IP: in.Body.IP, Name: in.Body.Name})
	if err != nil {
		return nil, clientsFailure(err)
	}
	return &reservationOutput{Body: r}, nil
}

func (s *server) deleteReservation(ctx context.Context, in *macPath) (*struct{ Body okBody }, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	if err := s.Clients.Unreserve(ctx, in.MAC); err != nil {
		return nil, clientsFailure(err)
	}
	return okResponse(), nil
}

func (s *server) setClientBlocked(ctx context.Context, in *blockInput) (*txnOutput, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	st, err := s.Clients.SetBlocked(ctx, in.MAC, in.Body.Blocked, caller)
	if err == nil || (st.ID != "" && (st.State == txn.RolledBack || st.State == txn.Restoring || st.State == txn.RollbackFailed)) {
		return txnResponse(st), nil
	}
	return nil, clientsFailure(err)
}

func (s *server) kickClient(ctx context.Context, in *macPath) (*struct{ Body okBody }, error) {
	if s.Clients == nil {
		return nil, clientsUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	if err := s.Clients.Kick(ctx, in.MAC, caller); err != nil {
		return nil, clientsFailure(err)
	}
	return okResponse(), nil
}
