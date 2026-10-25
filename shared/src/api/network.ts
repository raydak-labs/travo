import type { NetworkInterface, PortForwardRule } from '../generated/schemas';

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
  NetworkStatus,
  PortForwardRule,
  SetAliasRequest,
  SetInterfaceStateRequest,
  TrafficHistoryPoint,
  TrafficHistoryResponse,
  USBTetherStatus,
  UptimeEvent,
  WanConfig,
  WanDetectResult,
  WoLRequest,
} from '../generated/schemas';

export * from './failover';

/** Physical/logical medium an interface is attached to. */
export type NetworkMedium = 'ethernet' | 'wifi' | 'usb' | 'vpn' | 'lan' | 'unknown';

/**
 * Resolves the medium of an interface from its discriminator: wan, lan, wifi,
 * vpn or usb; the backend reports any other interface by its own name.
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

/** Request to add a port-forward rule */
export type AddPortForwardRequest = Omit<PortForwardRule, 'id'>;
