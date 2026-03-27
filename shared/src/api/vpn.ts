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
  /** Fine-grained tunnel state: disabled | configured | enabled_not_up | up_no_handshake | connected */
  readonly status_detail?: string;
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
export interface TailscalePeer {
  readonly hostname: string;
  readonly tailscale_ip: string;
  readonly os: string;
  readonly online: boolean;
  readonly exit_node: boolean;
  readonly exit_node_option: boolean;
  readonly last_seen: string;
}

export interface TailscaleStatus {
  readonly installed: boolean;
  readonly running: boolean;
  readonly logged_in: boolean;
  readonly ip_address: string;
  readonly hostname: string;
  readonly exit_node?: string;
  readonly exit_node_active: boolean;
  readonly peers: readonly TailscalePeer[];
  readonly auth_url?: string;
}

/** Live WireGuard peer status from `wg show` */
export interface WireGuardPeerStatus {
  readonly public_key: string;
  readonly endpoint: string;
  readonly latest_handshake: number; // unix epoch seconds, 0 = never
  readonly transfer_rx: number; // bytes received
  readonly transfer_tx: number; // bytes sent
  readonly allowed_ips: string;
}

/** Live WireGuard interface status from `wg show` */
export interface WireGuardStatus {
  readonly interface: string;
  readonly public_key: string;
  readonly listen_port: number;
  readonly peers: readonly WireGuardPeerStatus[];
}

/** Saved WireGuard profile */
export interface WireGuardProfile {
  readonly id: string;
  readonly name: string;
  readonly config: string;
  readonly active: boolean;
  readonly created_at: string;
}

/** VPN kill switch status */
export interface KillSwitchStatus {
  readonly enabled: boolean;
}

/** DNS leak test result */
export interface DNSLeakResult {
  /** Effective upstream nameservers (resolv.conf, or dnsmasq server= when resolv only lists local stub) */
  readonly nameservers: readonly string[];
  /** DNS servers configured in the active WireGuard profile */
  readonly vpn_dns_servers: readonly string[];
  /** True when a VPN tunnel is enabled */
  readonly vpn_active: boolean;
  /** True when VPN is active but nameservers don't match VPN-configured DNS */
  readonly potentially_leaking: boolean;
}

/** WireGuard tunnel verification result */
export interface VPNVerifyResult {
  readonly interface_up: boolean;
  readonly handshake_ok: boolean;
  readonly latest_handshake: number;
  readonly route_ok: boolean;
  readonly firewall_zone_ok: boolean;
  readonly forwarding_ok: boolean;
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

/** WireGuard split tunneling configuration */
export interface SplitTunnelConfig {
  readonly mode: 'all' | 'custom';
  readonly routes: readonly string[];
}

/** Tailscale SSH status */
export interface TailscaleSSHStatus {
  readonly enabled: boolean;
}
