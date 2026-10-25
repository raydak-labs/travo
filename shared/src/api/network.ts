import type { Client, NetworkInterface, PortForwardRule } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type {
  Client,
  ClientActionRequest,
  ConnectionMethod,
  DDNSConfig,
  DDNSConfigResponse,
  DDNSStatus,
  DHCPConfig,
  DHCPLease,
  DHCPReservation,
  DNSConfig,
  DNSEntry,
  DataBudget,
  DataBudgetConfig,
  DataUsageInterface,
  DataUsagePeriod,
  DataUsageStatus,
  DiagnosticsRequest,
  DiagnosticsResult,
  DoHConfig,
  FirewallZone,
  IPv6Status,
  NetworkInterface,
  PortForwardRule,
  SetAliasRequest,
  SetInterfaceStateRequest,
  TrafficHistoryPoint,
  TrafficHistoryResponse,
  USBTetherStatus,
  UptimeEvent,
  WanDetectResult,
  WoLRequest,
} from '../generated/schemas';

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

/** Overall network status */
export interface NetworkStatus {
  readonly wan: NetworkInterface | null;
  readonly lan: NetworkInterface;
  readonly interfaces: readonly NetworkInterface[];
  readonly clients: readonly Client[];
  readonly internet_reachable: boolean;
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

/** Request to add a port-forward rule */
export type AddPortForwardRequest = Omit<PortForwardRule, 'id'>;
