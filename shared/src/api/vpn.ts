import type { VpnStatus } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type {
  DNSLeakResult,
  KillSwitchStatus,
  SplitTunnelConfig,
  TailscalePeer,
  TailscaleStatus,
  VPNVerifyResult,
  VpnStatus,
  WireGuardPeerStatus,
  WireGuardProfile,
  WireGuardStatus,
  WireguardConfig,
  WireguardPeer,
} from '../generated/schemas';

/** VPN protocol type */
export type VpnType = 'wireguard' | 'openvpn' | 'tailscale';

/** Type guard for VpnStatus */
export function isVpnStatus(value: unknown): value is VpnStatus {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.type === 'string' &&
    typeof v.enabled === 'boolean' &&
    typeof v.connected === 'boolean' &&
    typeof v.connected_since === 'string' &&
    typeof v.endpoint === 'string' &&
    typeof v.rx_bytes === 'number' &&
    typeof v.tx_bytes === 'number'
  );
}

/** Tailscale SSH status */
export interface TailscaleSSHStatus {
  readonly enabled: boolean;
}
