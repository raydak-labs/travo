import { m } from '../paraglide/messages.js';
import { client } from './client';
import { type DoHView, getAdGuardDoh, setAdGuardDoh } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { DoHView };

/** Presets for AdGuard Home's upstream; any other https URL is "custom". */
export const dohPresets: [string, string][] = [
  ['Quad9', 'https://dns10.quad9.net/dns-query'],
  ['Cloudflare', 'https://cloudflare-dns.com/dns-query'],
  ['Google', 'https://dns.google/dns-query'],
];

/** Mirrors the daemon's check: https with a host, no credentials, query or whitespace. */
export function validDoH(raw: string): boolean {
  if (raw.length > 512 || /\s/.test(raw)) return false;
  try {
    const u = new URL(raw);
    return (
      u.protocol === 'https:' &&
      u.hostname !== '' &&
      !u.username &&
      !u.password &&
      !u.search &&
      !u.hash
    );
  } catch {
    return false;
  }
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function dohError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.doh_err_invalid();
    case 'doh_upstream_unreachable':
      return m.doh_err_unreachable();
    case 'doh_no_change':
      return m.doh_no_change();
    case 'adguard_busy':
      return m.adguard_err_busy();
    default:
      return m.doh_failed();
  }
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function fetchDoH(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DoHView> {
  const { data, response } = await getAdGuardDoh({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.doh_unavailable(), response?.status);
  return data;
}

export async function saveDoH(
  url: string,
  requestClient: Client = client,
): Promise<{ view: DoHView } | { error: string }> {
  try {
    const { data, error } = await setAdGuardDoh({ body: { url }, client: requestClient });
    return data ? { view: data } : { error: dohError(code(error)) };
  } catch {
    return { error: dohError(undefined) };
  }
}
