package api

import (
	"context"
	"net/http"
)

// WSTicketResponse is a short-lived, single-use WebSocket upgrade ticket.
type WSTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int64  `json:"expires_in"`
}

func registerWS(api *API, d *Dependencies) {
	post(api, "/api/v1/ws/ticket", "CreateWSTicket", "Issue a single-use WebSocket upgrade ticket",
		func(ctx context.Context, _ *none) (*Body[WSTicketResponse], error) {
			sessionToken := bearerToken(ctx)
			if sessionToken == "" {
				return nil, errorf(http.StatusUnauthorized, "missing session token")
			}
			ticket, expiresIn, err := d.WSTickets.Issue(sessionToken)
			if err != nil {
				return nil, serverError(err)
			}
			return reply(WSTicketResponse{Ticket: ticket, ExpiresIn: expiresIn})
		})
}
