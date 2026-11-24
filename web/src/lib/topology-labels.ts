import { formatBitRate } from './format';
import { m } from './paraglide/messages.js';
import type { SourceState, TopologyModel, TopologySource, UplinkName } from './topology';
import type { StatusKind } from './ui/status';

export const sourceLabel: Record<UplinkName, () => string> = {
  wan: m.failover_uplink_wan,
  wwan: m.failover_uplink_wwan,
  usbtether: m.failover_uplink_usb,
};

export const stateLabel: Record<SourceState, () => string> = {
  active: m.topology_state_active,
  standby: m.topology_state_standby,
  down: m.topology_state_down,
  not_configured: m.topology_state_not_configured,
  unknown: m.topology_state_unknown,
};

type Chip = { kind: StatusKind; label: () => string };

export const internetChip: Record<TopologyModel['internet'], Chip> = {
  online: { kind: 'ok', label: m.dashboard_internet_online },
  captive: { kind: 'warn', label: m.dashboard_internet_captive },
  offline: { kind: 'danger', label: m.dashboard_internet_offline },
  unknown: { kind: 'neutral', label: m.topology_state_unknown },
};

export const vpnChip: Record<TopologyModel['vpn'], Chip> = {
  connected: { kind: 'ok', label: m.vpn_state_connected },
  connecting: { kind: 'warn', label: m.topology_vpn_connecting },
  off: { kind: 'neutral', label: m.vpn_state_disabled },
  unknown: { kind: 'neutral', label: m.topology_state_unknown },
};

/** No chip when Tailscale is not installed. */
export const tailscaleChip: Record<Exclude<TopologyModel['tailscale'], 'absent'>, Chip> = {
  connected: { kind: 'ok', label: m.tailscale_state_connected },
  off: { kind: 'neutral', label: m.tailscale_state_off },
  unknown: { kind: 'neutral', label: m.topology_state_unknown },
};

/** "WAN: active, 12 Mbit/s down, 1 Mbit/s up" for the figure's text alternative. */
export function sourceSummary(source: TopologySource): string {
  const parts = [stateLabel[source.state]()];
  if (source.detail) parts.push(source.detail);
  if (source.rate) {
    parts.push(m.topology_rate_down({ rate: formatBitRate(source.rate.rxBitsPerSecond) }));
    parts.push(m.topology_rate_up({ rate: formatBitRate(source.rate.txBitsPerSecond) }));
  }
  return `${sourceLabel[source.name]()}: ${parts.join(', ')}`;
}

export function clientsSummary(clients: TopologyModel['clients']): string {
  if (clients === null) return `${m.nav_clients()}: ${m.topology_state_unknown()}`;
  const counts = `${m.topology_clients_wifi({ count: clients.wifi })}, ${m.topology_clients_lan({ count: clients.lan })}`;
  return `${m.nav_clients()}: ${counts}${clients.partial ? `, ${m.topology_clients_incomplete()}` : ''}`;
}

export function routerSummary(model: TopologyModel): string {
  const parts = [
    `${m.topology_internet()}: ${internetChip[model.internet].label()}`,
    `${m.dashboard_vpn()}: ${vpnChip[model.vpn].label()}`,
  ];
  if (model.tailscale !== 'absent') {
    parts.push(`${m.tailscale_title()}: ${tailscaleChip[model.tailscale].label()}`);
  }
  const name = model.router
    ? `${model.router.hostname} (${model.router.model})`
    : m.topology_router();
  return `${name}: ${parts.join(', ')}`;
}
