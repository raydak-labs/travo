import type { DashboardUplink } from './api/gen';
import { formatBitRate } from './format';
import { m } from './paraglide/messages.js';
import type { TopologySource } from './topology';
import { bandLabel } from './wifi';

const protoLabels: Record<string, () => string> = {
  dhcp: m.wan_proto_dhcp,
  static: m.wan_proto_static,
  pppoe: m.wan_proto_pppoe,
};

/** The label/value pairs the dashboard shows for one uplink (map hover card). */
export function uplinkFacts(
  source: TopologySource,
  raw: DashboardUplink | undefined,
): [string, string][] {
  const facts: [string, string][] = [];
  if (raw?.proto) facts.push([m.uplink_card_protocol(), protoLabels[raw.proto]?.() ?? raw.proto]);
  if (source.name !== 'wan' && source.detail) {
    facts.push([
      source.name === 'wwan' ? m.uplink_card_network() : m.uplink_card_link(),
      source.detail,
    ]);
  }
  if (raw?.band) facts.push([m.uplink_card_band(), bandLabel(raw.band)]);
  if (raw?.signal_dbm != null)
    facts.push([m.uplink_card_signal(), m.clients_signal({ signal: raw.signal_dbm })]);
  if (raw?.ipv4) facts.push([m.uplink_card_address(), raw.ipv4]);
  if (raw?.gateway) facts.push([m.uplink_card_gateway(), raw.gateway]);
  if (raw?.device) facts.push([m.uplink_card_device(), raw.device]);
  if (source.rate) {
    facts.push([
      m.uplink_card_traffic(),
      `↓ ${formatBitRate(source.rate.rxBitsPerSecond)} · ↑ ${formatBitRate(source.rate.txBitsPerSecond)}`,
    ]);
  }
  return facts;
}
