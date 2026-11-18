import { formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import type { StatusKind } from '../ui/status';
import { client } from './client';
import { type Interface, listFirewallZones, listNetworkInterfaces, type Zone } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { Interface as NetworkInterface, Zone as FirewallZone };

export type InterfaceList = { interfaces: Interface[]; partial: boolean };

export async function fetchInterfaces(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<InterfaceList> {
  const { data, response } = await listNetworkInterfaces({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.netview_unavailable(), response?.status);
  return data;
}

export async function fetchZones(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<Zone[]> {
  const { data, response } = await listFirewallZones({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.netview_zones_unavailable(), response?.status);
  return data.zones;
}

/** Up with its uptime, or down: a down uplink is a warning, any other down interface is neutral. */
export function interfaceStatus(it: Pick<Interface, 'up' | 'uptime_seconds' | 'uplink'>): {
  kind: StatusKind;
  label: string;
} {
  if (it.up)
    return { kind: 'ok', label: m.netview_up({ uptime: formatUptime(it.uptime_seconds) }) };
  return { kind: it.uplink ? 'warn' : 'neutral', label: m.netview_down() };
}

/** Addresses of both families, IPv4 first, for one line of the row. */
export function interfaceAddresses(it: Pick<Interface, 'ipv4' | 'ipv6'>): string {
  return [...it.ipv4, ...it.ipv6].join(', ');
}
