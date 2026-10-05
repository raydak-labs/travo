import type { NetworkInterface, NetworkInterfaceStats, VpnStatus } from '@shared/index';

/**
 * One vocabulary for the WAN uplinks, shared by the dashboard and the Network
 * Status page.
 *
 * They used to name the same three uplinks differently ("Repeater (WiFi)" vs
 * "WWAN (WiFi Client)") and describe the same disconnected state three ways
 * ("Repeater (STA) is disabled" / "Inactive" / "not configured"), so navigating
 * between the two pages meant re-learning the vocabulary.
 */
export interface UplinkSource {
  readonly key: 'wan' | 'wwan' | 'usbtether';
  /** Short label used on the dashboard. */
  readonly label: string;
  /** Fuller label used where the medium needs spelling out. */
  readonly longLabel: string;
  /** Shown when the uplink is present but down. */
  readonly inactiveCopy: string;
  /** Shown when the uplink was never configured. */
  readonly notConfiguredCopy: string;
  readonly description: string;
  readonly match: (iface: NetworkInterface) => boolean;
}

export const UPLINK_SOURCES: readonly UplinkSource[] = [
  {
    key: 'wan',
    label: 'Ethernet',
    longLabel: 'WAN (Ethernet)',
    inactiveCopy: 'Ethernet link is down.',
    notConfiguredCopy: 'No ethernet uplink configured.',
    description: 'Wired ethernet uplink via the WAN port.',
    match: (iface) => iface.type === 'wan' || (iface.name === 'wan' && iface.type !== 'wifi'),
  },
  {
    key: 'wwan',
    label: 'Repeater (WiFi)',
    longLabel: 'WWAN (WiFi Client)',
    inactiveCopy: 'Repeater (STA) is disabled.',
    notConfiguredCopy: 'Not configured as an upstream WiFi client.',
    description: 'Wireless uplink connected to an upstream WiFi network.',
    match: (iface) =>
      iface.type === 'wifi' && (iface.name.startsWith('wlan-sta') || iface.name.startsWith('wwan')),
  },
  {
    key: 'usbtether',
    label: 'USB Tethering',
    longLabel: 'USB Tethering',
    inactiveCopy: 'No tethering device found.',
    notConfiguredCopy: 'No USB uplink configured.',
    description: 'USB uplink from a tethered phone or modem.',
    match: (iface) => iface.type === 'usb' || iface.name === 'usbtether',
  },
];

export interface ResolvedUplink extends UplinkSource {
  readonly iface: NetworkInterface | undefined;
  /** Up *and* holding an address — a carrier without an address is not usable. */
  readonly active: boolean;
}

/** An interface counts as usable only once it holds an address. */
export function isUplinkActive(iface: NetworkInterface | undefined): boolean {
  return iface != null && iface.is_up && iface.ip_address !== '';
}

export function resolveUplinks(interfaces: readonly NetworkInterface[]): ResolvedUplink[] {
  return UPLINK_SOURCES.map((source) => {
    const iface = interfaces.find(source.match);
    return { ...source, iface, active: isUplinkActive(iface) };
  });
}

/**
 * Picks the interface whose throughput should be charted.
 *
 * The stats list is a fixed-order snapshot of a handful of interfaces
 * (`br-lan`, `wwan0`, `wg0`, `eth0` on the backend), so taking index 0 always
 * plotted LAN traffic no matter which uplink was actually carrying the
 * internet — while the card was titled "Network Throughput".
 *
 * Returns undefined when only the LAN bridge is available: there is no uplink
 * to plot, and charting the bridge would put LAN traffic under an "Internet
 * Throughput" title all over again.
 */
export function uplinkInterfaceName(
  names: readonly string[],
  uplinkName: string | null | undefined,
): string | undefined {
  if (names.length === 0) return undefined;
  if (uplinkName && names.includes(uplinkName)) return uplinkName;
  return names.find((name) => !name.startsWith('br-'));
}

/** As `uplinkInterfaceName`, over a stats payload. */
export function uplinkStatsInterface(
  stats: readonly NetworkInterfaceStats[] | undefined,
  uplinkName: string | null | undefined,
): NetworkInterfaceStats | undefined {
  if (!stats) return undefined;
  const name = uplinkInterfaceName(
    stats.map((s) => s.interface),
    uplinkName,
  );
  return name === undefined ? undefined : stats.find((s) => s.interface === name);
}

/**
 * Describes a tunnel as a user would recognise it.
 *
 * `enabled` and `connected` are separate fields, and `status_detail` exists to
 * tell them apart. Collapsing them to one boolean produced "VPN Off" sitting
 * directly above a "Disable VPN" button for a tunnel that was configured but
 * had not completed a handshake.
 */
export function describeVpnStatus(statuses: readonly VpnStatus[] | undefined): {
  label: string;
  active: boolean;
  /** False when the tunnel is configured but not carrying traffic. */
  healthy: boolean;
} {
  const connected = statuses?.find((s) => s.connected);
  if (connected) return { label: 'On', active: true, healthy: true };

  const enabled = statuses?.find((s) => s.enabled);
  if (!enabled) return { label: 'Off', active: false, healthy: false };

  switch (enabled.status_detail) {
    case 'up_no_handshake':
      return { label: 'Enabled, no handshake', active: true, healthy: false };
    case 'configured':
    case 'enabled_not_up':
      return { label: 'Enabled, not connected', active: true, healthy: false };
    default:
      return { label: 'Enabled', active: true, healthy: false };
  }
}

/** Short labels keyed by uplink, for call sites that do not need the whole row. */
export const UPLINK_LABELS: Readonly<Record<UplinkSource['key'], string>> = Object.fromEntries(
  UPLINK_SOURCES.map((s) => [s.key, s.label]),
) as Record<UplinkSource['key'], string>;

/** Copy shown for a present-but-down uplink. */
export const UPLINK_INACTIVE: Readonly<Record<UplinkSource['key'], string>> = Object.fromEntries(
  UPLINK_SOURCES.map((s) => [s.key, s.inactiveCopy]),
) as Record<UplinkSource['key'], string>;
