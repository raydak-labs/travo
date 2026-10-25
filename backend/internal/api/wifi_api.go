package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

// WifiApply is the pending rpcd apply a wireless change opened. The client
// confirms it with the token before RollbackTimeoutSeconds, leaving
// ProbeBudgetSeconds of room: one confirm can block that long while the device
// proves the new interfaces are up (ADR 0002 §5.1).
type WifiApply struct {
	Pending                bool   `json:"pending"`
	Token                  string `json:"token"`
	RollbackTimeoutSeconds int    `json:"rollback_timeout_seconds"`
	ProbeBudgetSeconds     int    `json:"probe_budget_seconds"`
}

// WifiMutationResponse is the envelope every wireless mutator answers with.
// GeneratedKey is set when the service had to invent a WPA passphrase.
type WifiMutationResponse struct {
	Status       string     `json:"status" example:"ok"`
	Apply        *WifiApply `json:"apply,omitempty"`
	GeneratedKey string     `json:"generated_key,omitempty"`
}

// RandomizeMACResponse adds the new MAC to the envelope.
type RandomizeMACResponse struct {
	WifiMutationResponse
	MAC string `json:"mac"`
}

// RepeaterOptionsResponse echoes the saved policy next to the envelope.
type RepeaterOptionsResponse struct {
	WifiMutationResponse
	AllowAPOnSTARadio bool `json:"allow_ap_on_sta_radio"`
}

// BandSwitchingResponse is the band switching config and its live state.
type BandSwitchingResponse struct {
	Config wifi.BandSwitchConfig `json:"config"`
	Status wifi.BandSwitchStatus `json:"status"`
}

// EnabledResponse reports a boolean state.
type EnabledResponse struct {
	Enabled bool `json:"enabled"`
}

// LockoutAck carries the explicit opt-in for a change the lockout guard would
// refuse because it disconnects the caller (ADR 0002).
type LockoutAck struct {
	AcknowledgeLockout bool `json:"acknowledge_lockout,omitempty"`
}

// SetModeRequest switches the WiFi operating mode.
type SetModeRequest struct {
	Mode string `json:"mode" enum:"ap,client,repeater"`
	LockoutAck
}

// SetRadioRequest enables or disables every radio.
type SetRadioRequest struct {
	Enabled bool `json:"enabled"`
	LockoutAck
}

// PriorityRequest orders the saved networks.
type PriorityRequest struct {
	SSIDs []string `json:"ssids"`
}

// ConfirmApplyRequest confirms a pending wireless apply.
type ConfirmApplyRequest struct {
	Token string `json:"token"`
}

type apSectionInput struct {
	Section string `path:"section"`
	Body    models.APConfigUpdate
}

type radioRoleInput struct {
	Name    string `path:"name"`
	Body    models.RadioRoleRequest
	RawBody []byte
}

func wifiMutation(apply *wifi.WirelessApplyResult) WifiMutationResponse {
	resp := WifiMutationResponse{Status: "ok"}
	if apply != nil {
		resp.Apply = &WifiApply{
			Pending:                true,
			Token:                  apply.Token,
			RollbackTimeoutSeconds: apply.RollbackTimeoutSeconds,
			ProbeBudgetSeconds:     apply.ProbeBudgetSeconds,
		}
		resp.GeneratedKey = apply.GeneratedKey
	}
	return resp
}

// mutated answers a wireless mutator: the envelope, or the error mapped to a
// status. A layout this hardware cannot run (ErrAPAndSTASameRadio), a guest
// subnet overlapping the LAN and a lockout refusal are 409 conflicts with the
// device's state, not server faults; the lockout carries wifi.LockoutErrorCode
// so the client raises its acknowledge dialog off the code.
func mutated(apply *wifi.WirelessApplyResult, err error) (*Body[WifiMutationResponse], error) {
	switch {
	case err == nil:
		return reply(wifiMutation(apply))
	case errors.Is(err, wifi.ErrLockoutRefused):
		return nil, errCode(http.StatusConflict, wifi.LockoutErrorCode, err.Error())
	case errors.Is(err, wifi.ErrAPAndSTASameRadio), errors.Is(err, wifi.ErrGuestSubnetOverlap):
		return nil, errorf(http.StatusConflict, err.Error())
	default:
		return nil, serverError(err)
	}
}

// lockout builds the guard input from the caller's address and its opt-in.
func lockout(ctx context.Context, acknowledged bool) wifi.LockoutRequest {
	return wifi.LockoutRequest{ClientIP: fiberCtx(ctx).IP(), AcknowledgeLockout: acknowledged}
}

func validateAPCredentials(ssid, encryption, key string) error {
	if strings.TrimSpace(ssid) == "" {
		return badRequest("ssid is required")
	}
	if encryption != "" && !isValidWifiEncryption(encryption) {
		return badRequest("encryption must be one of: none, psk2, sae, psk-mixed")
	}
	if encryption != "" && encryption != "none" && len(key) < 8 {
		return badRequest("password must be at least 8 characters")
	}
	return nil
}

func registerWifi(api *API, d *Dependencies) {
	svc := d.Wifi

	get(api, "/api/v1/wifi/scan", "WiFiScan", "Scan for networks (SSID, signal, encryption, band)",
		func(context.Context, *none) (*Body[[]models.WifiScanResult], error) { return replyOrFail(svc.Scan()) })
	post(api, "/api/v1/wifi/connect", "WiFiConnect", "Connect the uplink to a network",
		func(_ context.Context, in *Body[models.WifiConfig]) (*Body[WifiMutationResponse], error) {
			cfg := in.Body
			if strings.TrimSpace(cfg.SSID) == "" {
				return nil, badRequest("SSID must not be empty")
			}
			if cfg.Encryption != "none" && cfg.Password != "" && len(cfg.Password) < 8 {
				return nil, badRequest("password must be at least 8 characters for WPA networks")
			}
			apply, err := svc.Connect(cfg)
			if errors.Is(err, wifi.ErrPasswordRequiredForNewSTA) || errors.Is(err, wifi.ErrEncryptionRequiredForNewSTA) {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return mutated(apply, err)
		})
	post(api, "/api/v1/wifi/disconnect", "WiFiDisconnect", "Disconnect the uplink",
		func(context.Context, *none) (*Body[WifiMutationResponse], error) {
			return mutatedPlain(svc.Disconnect())
		})
	get(api, "/api/v1/wifi/connection", "GetWiFiConnection", "Uplink connection state",
		func(context.Context, *none) (*Body[models.WifiConnection], error) {
			return replyOrFail(svc.GetConnection())
		})
	get(api, "/api/v1/wifi/health", "GetWiFiHealth", "Uplink health and detected issues",
		func(context.Context, *none) (*Body[models.WifiHealth], error) { return replyOrFail(svc.GetHealth()) })
	put(api, "/api/v1/wifi/mode", "SetWiFiMode", "Switch the WiFi mode",
		func(ctx context.Context, in *strictBody[SetModeRequest]) (*Body[WifiMutationResponse], error) {
			if err := in.decode(); err != nil {
				return nil, err
			}
			if strings.TrimSpace(in.Body.Mode) == "" {
				return nil, badRequest("mode is required")
			}
			return mutated(svc.SetMode(in.Body.Mode, lockout(ctx, in.Body.AcknowledgeLockout)))
		}, describe("Client mode removes every access point: a caller on WiFi gets 409 "+
			"wifi_lockout_risk unless acknowledge_lockout is set."))

	get(api, "/api/v1/wifi/saved", "GetSavedNetworks", "Saved uplink networks",
		func(context.Context, *none) (*Body[[]models.SavedNetwork], error) {
			networks, err := svc.GetSavedNetworks()
			if networks == nil {
				networks = []models.SavedNetwork{}
			}
			return replyOrFail(networks, err)
		})
	del(api, "/api/v1/wifi/saved/{section}", "DeleteSavedNetwork", "Delete a saved network",
		func(_ context.Context, in *sectionParam) (*Body[WifiMutationResponse], error) {
			return mutatedPlain(svc.DeleteNetwork(in.Section))
		})
	put(api, "/api/v1/wifi/saved/priority", "SetNetworkPriority", "Order the saved networks",
		func(_ context.Context, in *Body[PriorityRequest]) (*Body[Status], error) {
			if len(in.Body.SSIDs) == 0 {
				return nil, badRequest("ssids list must not be empty")
			}
			return okOrFail(svc.ReorderNetworks(in.Body.SSIDs))
		})

	get(api, "/api/v1/wifi/radio", "GetRadioStatus", "Whether the radios are enabled",
		func(context.Context, *none) (*Body[EnabledResponse], error) {
			enabled, err := svc.GetRadioStatus()
			return replyOrFail(EnabledResponse{Enabled: enabled}, err)
		})
	put(api, "/api/v1/wifi/radio", "SetRadioEnabled", "Enable or disable every radio",
		func(ctx context.Context, in *strictBody[SetRadioRequest]) (*Body[WifiMutationResponse], error) {
			if err := in.decode(); err != nil {
				return nil, err
			}
			return mutated(svc.SetRadioEnabled(in.Body.Enabled, lockout(ctx, in.Body.AcknowledgeLockout)))
		})
	get(api, "/api/v1/wifi/radios", "GetRadios", "Radio hardware (band, channel, role)",
		func(context.Context, *none) (*Body[[]models.RadioInfo], error) { return replyOrFail(svc.GetRadios()) })
	put(api, "/api/v1/wifi/radios/{name}/role", "SetRadioRole", "Assign a radio to AP, uplink or both",
		func(ctx context.Context, in *radioRoleInput) (*Body[WifiMutationResponse], error) {
			var req models.RadioRoleRequest
			if err := decodeStrictJSON(in.RawBody, &req, ""); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return mutated(svc.SetRadioRole(in.Name, req.Role, lockout(ctx, req.AcknowledgeLockout)))
		})

	get(api, "/api/v1/wifi/ap", "GetAPConfig", "Access point configuration of every radio",
		func(context.Context, *none) (*Body[[]models.APConfig], error) { return replyOrFail(svc.GetAPConfigs()) })
	put(api, "/api/v1/wifi/ap/{section}", "SetAPConfig", "Update one access point",
		func(ctx context.Context, in *apSectionInput) (*Body[WifiMutationResponse], error) {
			u := in.Body
			if err := validateAPCredentials(u.SSID, u.Encryption, u.Key); err != nil {
				return nil, err
			}
			return mutated(svc.SetAPConfig(in.Section, u, lockout(ctx, u.AcknowledgeLockout)))
		})
	get(api, "/api/v1/wifi/guest", "GetGuestWifi", "Guest network configuration",
		func(context.Context, *none) (*Body[*models.GuestWifiConfig], error) {
			return replyOrFail(svc.GetGuestWifi())
		})
	put(api, "/api/v1/wifi/guest", "SetGuestWifi", "Set the guest network",
		func(ctx context.Context, in *strictBody[models.GuestWifiConfig]) (*Body[WifiMutationResponse], error) {
			if err := in.decode(); err != nil {
				return nil, err
			}
			cfg := in.Body
			if cfg.Enabled {
				if err := validateAPCredentials(cfg.SSID, cfg.Encryption, cfg.Key); err != nil {
					return nil, err
				}
			}
			return mutated(svc.SetGuestWifi(cfg, lockout(ctx, cfg.AcknowledgeLockout)))
		})

	get(api, "/api/v1/wifi/repeater-options", "GetRepeaterOptions", "Repeater radio policy",
		func(context.Context, *none) (*Body[models.RepeaterOptions], error) {
			return replyOrFail(svc.GetRepeaterOptions())
		})
	put(api, "/api/v1/wifi/repeater-options", "SetRepeaterOptions", "Set the repeater radio policy",
		func(_ context.Context, in *Body[models.RepeaterOptions]) (*Body[RepeaterOptionsResponse], error) {
			apply, err := svc.SetRepeaterOptions(in.Body)
			if err != nil {
				return nil, serverError(err)
			}
			return reply(RepeaterOptionsResponse{WifiMutationResponse: wifiMutation(apply), AllowAPOnSTARadio: in.Body.AllowAPOnSTARadio})
		})
	post(api, "/api/v1/wifi/repeater/reconcile", "ReconcileRepeaterAPLayout", "Move access points off the uplink radio",
		func(context.Context, *none) (*Body[WifiMutationResponse], error) {
			apply, err := svc.ReconcileRepeaterAPLayout()
			if err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return reply(wifiMutation(apply))
		})

	get(api, "/api/v1/wifi/band-switching", "GetBandSwitching", "Automatic band switching config and state",
		func(context.Context, *none) (*Body[BandSwitchingResponse], error) {
			return reply(BandSwitchingResponse{Config: d.BandSwitching.GetConfig(), Status: d.BandSwitching.GetStatus()})
		})
	put(api, "/api/v1/wifi/band-switching", "SetBandSwitching", "Set automatic band switching",
		func(_ context.Context, in *strictBody[wifi.BandSwitchConfig]) (*Body[Status], error) {
			cfg, err := decodeBandSwitchConfig(in.RawBody)
			if err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			if err := wifi.ValidateBandSwitchConfig(cfg); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return okOrFail(d.BandSwitching.SetConfig(cfg))
		}, describe("Accepts the bare config or the {config} envelope GET returns."))

	get(api, "/api/v1/wifi/mac", "GetMAC", "Interface MAC addresses",
		func(context.Context, *none) (*Body[[]models.MACConfig], error) {
			return replyOrFail(svc.GetMACAddresses())
		})
	put(api, "/api/v1/wifi/mac", "SetMAC", "Set or reset the uplink MAC address",
		func(_ context.Context, in *Body[models.SetMACRequest]) (*Body[WifiMutationResponse], error) {
			if in.Body.MAC != "" && !isValidMAC(in.Body.MAC) {
				return nil, badRequest("invalid MAC address format (expected XX:XX:XX:XX:XX:XX)")
			}
			return mutatedPlain(svc.SetMACAddress(in.Body.MAC))
		})
	post(api, "/api/v1/wifi/mac/randomize", "RandomizeMAC", "Randomize the uplink MAC address",
		func(context.Context, *none) (*Body[RandomizeMACResponse], error) {
			mac, apply, err := svc.RandomizeMAC()
			if err != nil {
				return nil, serverError(err)
			}
			return reply(RandomizeMACResponse{WifiMutationResponse: wifiMutation(apply), MAC: mac})
		})
	get(api, "/api/v1/wifi/mac-policies", "GetMACPolicies", "Per-network MAC policies",
		func(context.Context, *none) (*Body[models.MACPolicies], error) {
			return replyOrFail(svc.GetMACPolicies())
		})
	put(api, "/api/v1/wifi/mac-policies", "SetMACPolicies", "Set the per-network MAC policies",
		strict(func(_ context.Context, p models.MACPolicies) (*Body[Status], error) {
			return okOrFail(svc.SetMACPolicies(p))
		}))

	get(api, "/api/v1/wifi/autoreconnect", "GetAutoReconnect", "Whether the uplink reconnects automatically",
		func(context.Context, *none) (*Body[EnabledResponse], error) {
			enabled, err := svc.GetAutoReconnect()
			return replyOrFail(EnabledResponse{Enabled: enabled}, err)
		})
	put(api, "/api/v1/wifi/autoreconnect", "SetAutoReconnect", "Enable or disable automatic reconnect",
		func(_ context.Context, in *Body[EnabledRequest]) (*Body[Status], error) {
			return okOrFail(svc.SetAutoReconnect(in.Body.Enabled))
		})
	get(api, "/api/v1/wifi/schedule", "GetWiFiSchedule", "WiFi on/off schedule",
		func(context.Context, *none) (*Body[models.WiFiSchedule], error) {
			return replyOrFail(svc.GetWiFiSchedule())
		})
	put(api, "/api/v1/wifi/schedule", "SetWiFiSchedule", "Set the WiFi on/off schedule",
		strict(func(_ context.Context, s models.WiFiSchedule) (*Body[Status], error) {
			// The times go into a root crontab line.
			if s.Enabled && s.OnTime != "" && s.OffTime != "" {
				if err := platform.ValidateHHMM(s.OnTime); err != nil {
					return nil, badRequest("on_time: " + err.Error())
				}
				if err := platform.ValidateHHMM(s.OffTime); err != nil {
					return nil, badRequest("off_time: " + err.Error())
				}
			}
			return okOrFail(svc.SetWiFiSchedule(s))
		}))

	post(api, "/api/v1/wifi/apply/confirm", "ConfirmWiFiApply", "Confirm a pending wireless apply",
		func(_ context.Context, in *Body[ConfirmApplyRequest]) (*Body[Status], error) {
			if strings.TrimSpace(in.Body.Token) == "" {
				return nil, badRequest("token is required")
			}
			return okOrFail(svc.ConfirmApply(in.Body.Token))
		}, describe("Blocks up to probe_budget_seconds while the device proves the new interfaces are up. "+
			`"wireless apply not verified:" means they did not come up and the change is being reverted; `+
			`"wireless apply could not be verified:" means the answer could not be read. Clients key on both prefixes.`))
}

// mutatedPlain answers a mutator whose errors are all server faults.
func mutatedPlain(apply *wifi.WirelessApplyResult, err error) (*Body[WifiMutationResponse], error) {
	if err != nil {
		return nil, serverError(err)
	}
	return reply(wifiMutation(apply))
}

// decodeBandSwitchConfig strictly decodes the bare config or the {config}
// envelope that GET returns, whichever the body uses.
func decodeBandSwitchConfig(raw []byte) (wifi.BandSwitchConfig, error) {
	var probe struct {
		Config *wifi.BandSwitchConfig `json:"config"`
	}
	var cfg wifi.BandSwitchConfig
	if err := json.Unmarshal(raw, &probe); err != nil {
		return cfg, err
	}
	wrapper := ""
	if probe.Config != nil {
		wrapper = "config"
	}
	err := decodeStrictJSON(raw, &cfg, wrapper)
	return cfg, err
}
