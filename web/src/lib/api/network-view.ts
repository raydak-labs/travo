import { formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import type { StatusKind } from '../ui/status';
import { client } from './client';
import {
  type Interface,
  listFirewallZones,
  listNetworkInterfaces,
  restartNetworkInterface,
  type Zone,
} from './gen';
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

/** The uplinks the daemon restarts; wan6 follows wan, LAN and guest never restart (lockout). */
export const RESTARTABLE = ['wan', 'wwan', 'usbtether'] as const;
export type Restartable = (typeof RESTARTABLE)[number];

export function restartable(name: string): name is Restartable {
  return (RESTARTABLE as readonly string[]).includes(name);
}

export type RestartOutcome = { up: boolean } | { lockout: true } | { error: string };

/** Maps the API's stable error codes to a message; never parses error text. */
export function restartError(code: string | undefined): string {
  switch (code) {
    case 'transaction_pending':
      return m.netview_restart_err_pending();
    case 'interface_restart_busy':
      return m.netview_restart_err_busy();
    case 'interface_not_found':
      return m.netview_restart_err_missing();
    case 'interface_restart_failed':
      return m.netview_restart_err_failed();
    default:
      return m.netview_restart_err_unavailable();
  }
}

/** Restarts one uplink; a lockout refusal is its own outcome so the page can offer the override. */
export async function restartUplink(
  name: Restartable,
  acknowledge = false,
  requestClient: Client = client,
): Promise<RestartOutcome> {
  try {
    const { data, error } = await restartNetworkInterface({
      path: { name },
      body: acknowledge ? { acknowledge_lockout: true } : undefined,
      client: requestClient,
    });
    if (data) return { up: data.up };
    const code = (error as { code?: string } | undefined)?.code;
    return code === 'wifi_lockout_risk' ? { lockout: true } : { error: restartError(code) };
  } catch {
    return { error: restartError(undefined) };
  }
}
