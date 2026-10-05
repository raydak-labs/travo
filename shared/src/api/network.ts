/** Single connectivity state transition event */
export interface UptimeEvent {
  readonly timestamp: number; // Unix milliseconds
  readonly state: 'connected' | 'disconnected';
}

/**
 * Discriminator for a network interface.
 *
 * The router reports the *medium/role* of an interface, never its UCI
 * section name: a wired uplink is `wan`, the bridge is `lan`, a wireless
 * interface (AP or STA) is `wifi`, a tunnel is `vpn` and a USB-tethered
 * uplink is `usb`. Consumers must therefore not read the connection protocol
 * (dhcp / static / pppoe) from here — that lives in `WanConfig.type`.
 */
export type NetworkInterfaceType = 'wan' | 'lan' | 'wifi' | 'vpn' | 'usb';

/** Network interface details */
export interface NetworkInterface {
  readonly name: string;
  readonly type: NetworkInterfaceType;
  readonly ip_address: string;
  readonly netmask: string;
  readonly gateway: string;
  readonly dns_servers: readonly string[];
  readonly mac_address: string;
  readonly is_up: boolean;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
}

/** Connection method for the current client */
export interface ConnectionMethod {
  readonly method: 'wifi-client' | 'wifi-ap' | 'ethernet' | 'unknown';
  readonly interface: string;
  readonly ip_address: string;
}

/** WAN connection type */
export type WanType = 'dhcp' | 'static' | 'pppoe' | 'usb_tethering' | 'none';

/** WAN configuration */
export interface WanConfig {
  readonly type: WanType;
  readonly interface_name: string;
  readonly ip_address: string;
  readonly netmask: string;
  readonly gateway: string;
  readonly dns_servers: readonly string[];
  readonly mtu: number;
}

/** Connected client */
export interface Client {
  readonly ip_address: string;
  readonly mac_address: string;
  readonly hostname: string;
  readonly alias?: string;
  readonly interface_name: string;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
  readonly connected_since: string;
}

/** Request to set a device alias */
export interface SetAliasRequest {
  readonly mac: string;
  readonly alias: string;
}

/** Request body for kick/block/unblock client actions */
export interface ClientActionRequest {
  readonly mac: string;
}

/** Request to set interface up/down state */
export interface SetInterfaceStateRequest {
  readonly up: boolean;
}

/** Overall network status */
export interface NetworkStatus {
  readonly wan: NetworkInterface | null;
  readonly lan: NetworkInterface;
  readonly interfaces: readonly NetworkInterface[];
  readonly clients: readonly Client[];
  readonly internet_reachable: boolean;
}

/** DHCP server configuration for LAN */
export interface DHCPConfig {
  readonly start: number;
  readonly limit: number;
  readonly lease_time: string;
}

/** Custom DNS server configuration */
export interface DNSConfig {
  readonly use_custom_dns: boolean;
  readonly servers: readonly string[];
}

/** Active DHCP lease */
export interface DHCPLease {
  readonly expiry: number;
  readonly mac: string;
  readonly ip: string;
  readonly hostname: string;
}

/** Local DNS entry (hostname → IP mapping) */
export interface DNSEntry {
  readonly name: string;
  readonly ip: string;
  readonly section?: string;
}

/** WAN connection type auto-detection result */
export interface WanDetectResult {
  readonly detected_type: WanType;
  readonly current_type: WanType;
}

/** Static DHCP reservation (IP by MAC) */
export interface DHCPReservation {
  readonly name: string;
  readonly mac: string;
  readonly ip: string;
  readonly section?: string;
}

/** Dynamic DNS provider configuration */
export interface DDNSConfig {
  readonly enabled: boolean;
  readonly service: string;
  readonly domain: string;
  readonly username: string;
  readonly password: string;
  readonly lookup_host: string;
  /** Custom ddns-scripts update URL when `service` is `"custom"` (OpenWrt UCI `update_url`). */
  readonly update_url: string;
}

/**
 * GET /network/ddns response.
 *
 * `available` is false when the ddns-scripts package is not installed: nothing
 * on the router can then service a `ddns` config, and ddns is not in the service
 * catalog, so the UI cannot offer an install. The UI uses this to explain that
 * instead of presenting a form whose only possible outcome is a 503.
 */
export interface DDNSConfigResponse {
  readonly config: DDNSConfig;
  readonly available: boolean;
}

/** Dynamic DNS service status */
export interface DDNSStatus {
  readonly running: boolean;
  readonly public_ip: string;
  readonly last_update: string;
}

/** RX/TX bytes for a specific time period */
export interface DataUsagePeriod {
  readonly rx_bytes: number;
  readonly tx_bytes: number;
}

/** One cumulative RX/TX sample for a single interface (GET /network/traffic-history) */
export interface TrafficHistoryPoint {
  /** Unix timestamp in seconds */
  readonly t: number;
  readonly ifname: string;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
}

/**
 * Recent per-interface traffic samples so the chart has something to paint on
 * page load. `points` is empty right after boot; `retained_seconds` is how much
 * history the server actually holds (~600s at the 2s sampling tick).
 */
export interface TrafficHistoryResponse {
  readonly points: readonly TrafficHistoryPoint[];
  readonly retained_seconds: number;
}

/** Traffic data for a single monitored network interface */
export interface DataUsageInterface {
  readonly name: string;
  readonly label: string;
  readonly today: DataUsagePeriod;
  readonly month: DataUsagePeriod;
  readonly total: DataUsagePeriod;
}

/** Top-level data usage response (available=false when vnstat not installed) */
export interface DataUsageStatus {
  readonly available: boolean;
  readonly interfaces: readonly DataUsageInterface[];
}

/** Monthly usage budget for a single interface */
export interface DataBudget {
  readonly interface: string;
  readonly monthly_limit_bytes: number;
  readonly warning_threshold_pct: number;
  readonly reset_day: number;
}

/** All configured data budgets */
export interface DataBudgetConfig {
  readonly budgets: readonly DataBudget[];
}

/** USB tethering detection and configuration status */
export interface USBTetherStatus {
  readonly detected: boolean;
  readonly device_type: 'android' | 'ios' | 'unknown' | '';
  readonly interface: string;
  readonly is_up: boolean;
  readonly ip_address: string;
  readonly configured: boolean;
}

export * from './failover';

/** Physical/logical medium an interface is attached to. */
export type NetworkMedium = 'ethernet' | 'wifi' | 'usb' | 'vpn' | 'lan' | 'unknown';

/**
 * Resolves the medium of an interface from its discriminator.
 *
 * `type` is authoritative on current firmware. The name-based fallback keeps
 * the dashboard honest against firmware that still reports the raw ubus
 * section name in `type` (e.g. `wwan`, `usb0`, `wg0`) instead of the
 * normalised discriminator.
 */
export function networkMedium(
  iface: Pick<NetworkInterface, 'name' | 'type'> | null | undefined,
): NetworkMedium {
  if (!iface) return 'unknown';
  switch (iface.type) {
    case 'wan':
      return 'ethernet';
    case 'lan':
      return 'lan';
    case 'wifi':
      return 'wifi';
    case 'vpn':
      return 'vpn';
    case 'usb':
      return 'usb';
    default:
      return mediumFromName(iface.name);
  }
}

function mediumFromName(name: string): NetworkMedium {
  const n = name.toLowerCase();
  if (n.startsWith('wlan') || n.startsWith('wwan') || n.startsWith('ath')) return 'wifi';
  if (n.startsWith('usb') || n.startsWith('rndis') || n.startsWith('cdc') || n.startsWith('ncm'))
    return 'usb';
  if (n.startsWith('wg') || n.startsWith('tailscale') || n.startsWith('utun')) return 'vpn';
  if (n.startsWith('br-') || n === 'lan') return 'lan';
  if (n === 'wan' || n.startsWith('eth')) return 'ethernet';
  return 'unknown';
}

/** Type guard for NetworkStatus */
export function isNetworkStatus(value: unknown): value is NetworkStatus {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    'wan' in v &&
    typeof v.lan === 'object' &&
    v.lan !== null &&
    Array.isArray(v.interfaces) &&
    Array.isArray(v.clients) &&
    typeof v.internet_reachable === 'boolean'
  );
}

/** A summary of a UCI firewall zone */
export interface FirewallZone {
  readonly name: string;
  readonly input: string;
  readonly output: string;
  readonly forward: string;
  readonly network: readonly string[];
}

/** A DNAT port-forward rule */
export interface PortForwardRule {
  readonly id: string;
  readonly name: string;
  readonly protocol: string;
  readonly src_dport: string;
  readonly dest_ip: string;
  readonly dest_port: string;
  readonly enabled: boolean;
}

/** Request to add a port-forward rule */
export type AddPortForwardRequest = Omit<PortForwardRule, 'id'>;

/** Request to send a Wake-on-LAN magic packet */
export interface WoLRequest {
  readonly mac: string;
  readonly interface?: string;
}

/** DNS-over-HTTPS configuration */
export interface DoHConfig {
  readonly enabled: boolean;
  readonly provider: 'cloudflare' | 'google' | 'quad9' | 'custom';
  readonly url: string;
}

/** IPv6 status and global addresses */
export interface IPv6Status {
  readonly enabled: boolean;
  readonly addresses: readonly string[];
}

/** Request to run a network diagnostic */
export interface DiagnosticsRequest {
  readonly type: 'ping' | 'traceroute' | 'dns';
  readonly target: string;
}

/** Result of a network diagnostic command */
export interface DiagnosticsResult {
  readonly type: string;
  readonly target: string;
  readonly output: string;
  readonly error?: string;
}
