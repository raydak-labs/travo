import { m } from '../paraglide/messages.js';
import { client } from './client';
import { type Interface, type Wake, wakeOnLan } from './gen';
import type { Client } from './gen/client';

export type { Wake };

export type WakeOutcome = { wake: Wake } | { error: string };

/** Mirrors the daemon's MAC rule: six hex pairs with ':' or '-', unicast, not zero. */
export function validWakeMAC(mac: string): boolean {
  const s = mac.trim();
  if (!/^[0-9a-f]{2}([:-])[0-9a-f]{2}(\1[0-9a-f]{2}){4}$/i.test(s)) return false;
  if (/^0+$/.test(s.replace(/[:-]/g, ''))) return false;
  return (Number.parseInt(s.slice(0, 2), 16) & 1) === 0;
}

/** The interfaces a wake may target: up, non-uplink, on a bridge with an IPv4 address. */
export function wakeInterfaces(
  list: Pick<Interface, 'name' | 'up' | 'uplink' | 'device' | 'ipv4'>[],
): string[] {
  return list
    .filter((it) => it.up && !it.uplink && it.device?.startsWith('br-') && it.ipv4.length > 0)
    .map((it) => it.name);
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function wakeError(code: string | undefined): string {
  switch (code) {
    case 'invalid_mac':
      return m.wol_err_mac();
    case 'invalid_interface':
      return m.wol_err_interface();
    case 'wol_busy':
      return m.wol_err_busy();
    case 'rate_limited':
      return m.wol_err_rate();
    default:
      return m.wol_err_failed();
  }
}

export async function wake(
  mac: string,
  iface = '',
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<WakeOutcome> {
  try {
    const { data, error } = await wakeOnLan({
      body: iface ? { mac: mac.trim(), interface: iface } : { mac: mac.trim() },
      signal,
      client: requestClient,
    });
    if (data) return { wake: data };
    return { error: wakeError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: wakeError(undefined) };
  }
}
