package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

// wifiMutationResponse is the single response envelope every wireless mutator
// returns, so the client can find the pending apply in one place
// (response.apply) and, when the service had to invent a WPA passphrase, show it
// next to it (response.generated_key) instead of losing it with the rollback.
func wifiMutationResponse(apply *wifi.WirelessApplyResult) fiber.Map {
	resp := fiber.Map{"status": "ok"}
	if apply != nil {
		resp["apply"] = fiber.Map{
			"pending":                  true,
			"token":                    apply.Token,
			"rollback_timeout_seconds": apply.RollbackTimeoutSeconds,
		}
		// How long one confirm call can block on the device while it waits for the
		// new interfaces to come up. A client that re-POSTs confirm until the
		// rollback deadline has to leave this much room, or a probe it starts near
		// the deadline is answered after rpcd has already rolled back. The value
		// is the worst-case blocking time (retries AND the ubus round-trips of
		// the per-interface fallback), not the sum of the retry sleeps; see
		// wirelessProbeBudgetSeconds and ADR 0002 §5.1.
		//
		// Emitted unconditionally: a key that appears only when non-zero is a
		// worse contract than one that is always present, because a generated
		// client cannot rely on finding it.
		resp["apply"].(fiber.Map)["probe_budget_seconds"] = apply.ProbeBudgetSeconds
		if apply.GeneratedKey != "" {
			resp["generated_key"] = apply.GeneratedKey
		}
	}
	return resp
}

// respondWifiMutationError maps a wireless mutator's error to a status code.
// ErrAPAndSTASameRadio is a 409: this radio layout is not possible on this
// hardware, which is a conflict with the device's current state, not a server
// failure. Answering 500 would tell the operator (and the frontend's retry
// policy) that Travo itself broke.
// ErrGuestSubnetOverlap is a 409 for the same reason: the guest subnet collides
// with network.lan, so the request conflicts with the current configuration.
// ErrLockoutRefused is a 409 too, and carries wifi.LockoutErrorCode in the
// body so the frontend can raise its acknowledge-the-lockout dialog off the
// code rather than off this message.
func respondWifiMutationError(c fiber.Ctx, err error) error {
	if errors.Is(err, wifi.ErrLockoutRefused) {
		return RespondWithErrorCode(c, fiber.StatusConflict,
			wifi.LockoutErrorCode, err.Error())
	}
	if errors.Is(err, wifi.ErrAPAndSTASameRadio) ||
		errors.Is(err, wifi.ErrGuestSubnetOverlap) {
		return RespondWithError(c, fiber.StatusConflict, err.Error())
	}
	return RespondWithServerError(c, err)
}

// lockoutRequest builds the guard input for a mutating wireless request from
// the caller's address and the acknowledgement flag in its body.
func lockoutRequest(c fiber.Ctx, acknowledged bool) wifi.LockoutRequest {
	return wifi.LockoutRequest{ClientIP: c.IP(), AcknowledgeLockout: acknowledged}
}

// ConfirmWifiApplyHandler handles POST /api/v1/wifi/apply/confirm.
//
// The call is not cheap: ConfirmApply proves on the device that the interfaces
// the applied config enables are really up, so one call can block for up to
// ProbeBudgetSeconds (see wifiMutationResponse). A client that re-POSTs confirm
// until the rollback deadline has to leave that much room, or a probe it starts
// near the deadline is only answered after rpcd has already rolled back.
//
// A failed proof returns before applier.Confirm, so rpcd's rollback window is
// still open. Both failure modes are 5xx with a message that says which one it
// is: "wireless apply not verified:" means the access points did not come up and
// the change is being reverted, "wireless apply could not be verified:" means
// this build could not read netifd's answer and nothing is known about the
// change yet. The client keys on those two prefixes; renaming one without the
// other turns a diagnostic into a false "the router rolled back" on screen.
func ConfirmWifiApplyHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Token string `json:"token"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if strings.TrimSpace(body.Token) == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "token is required")
		}
		if err := svc.ConfirmApply(body.Token); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
