/** VPN protocol type */
export type VpnType = 'wireguard' | 'openvpn' | 'tailscale';

/** VPN connection status */
export interface VpnStatus {
  readonly type: VpnType;
  readonly enabled: boolean;
  readonly connected: boolean;
  readonly connected_since: string;
  readonly endpoint: string;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
}

/** WireGuard configuration */
export interface WireguardConfig {
  readonly private_key: string;
  readonly address: string;
  readonly dns: readonly string[];
  readonly peers: readonly WireguardPeer[];
}

/** WireGuard peer */
export interface WireguardPeer {
  readonly public_key: string;
  readonly endpoint: string;
  readonly allowed_ips: readonly string[];
  readonly preshared_key?: string;
  readonly last_handshake?: string;
}

/** Tailscale status */
export interface TailscaleStatus {
  readonly installed: boolean;
  readonly running: boolean;
  readonly logged_in: boolean;
  readonly ip_address: string;
  readonly hostname: string;
  readonly exit_node?: string;
  readonly exit_node_active: boolean;
}

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
