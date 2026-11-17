package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestWSTicketEndpointIssuesSingleUseTicket(t *testing.T) {
	app, deps := setupTestApp(t)
	sessionToken, _, err := deps.Auth.Login("admin")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/ws/ticket", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("ticket request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ticket status = %d: %s", resp.StatusCode, body)
	}
	var ticket struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &ticket); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}
	if len(ticket.Ticket) != 64 || ticket.ExpiresIn != 30 {
		t.Errorf("unexpected ticket response: %+v", ticket)
	}
	if got, ok := deps.WSTickets.Consume(ticket.Ticket); !ok || got != sessionToken {
		t.Errorf("ticket is not bound to the authenticated session")
	}
	if _, ok := deps.WSTickets.Consume(ticket.Ticket); ok {
		t.Error("endpoint ticket could be consumed twice")
	}
}

func TestWSTicketEndpointRequiresAuth(t *testing.T) {
	app, _ := setupTestApp(t)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/ws/ticket", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("ticket request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated ticket status = %d, want 401", resp.StatusCode)
	}
}
