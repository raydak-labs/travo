import type { TailscalePhase } from '../api/tailscale';
import { m } from '../paraglide/messages.js';
import type { StatusKind } from '../ui/status';

/** One label and tone per Tailscale phase, shared by the VPN overview and the Tailscale page. */
export const tailscaleStates: Record<TailscalePhase, { kind: StatusKind; label: () => string }> = {
  absent: { kind: 'neutral', label: m.tailscale_state_absent },
  stopped: { kind: 'neutral', label: m.tailscale_state_off },
  login: { kind: 'warn', label: m.tailscale_state_login },
  connected: { kind: 'ok', label: m.tailscale_state_connected },
  off: { kind: 'neutral', label: m.tailscale_state_off },
  starting: { kind: 'info', label: m.tailscale_state_starting },
};
