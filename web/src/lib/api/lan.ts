import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getLanAddress, type LanPending, type LanView, type TxnBody } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { LanPending, LanView };

/** Seconds the old-address page counts down when the PUT never answered (window + apply). */
export const LOST_ANSWER_SECONDS = 180 + 65;

/** How long the old-address page waits for the PUT before it assumes the address moved. */
export const ANSWER_WAIT_MS = 10_000;

/** The page to open on the new address: same scheme and port, the LAN page. */
export function moveURL(address: string, at: { protocol: string; port: string }): string {
  const ip = address.split('/')[0] ?? address;
  return `${at.protocol}//${ip}${at.port ? `:${at.port}` : ''}/#/network/lan`;
}

/** Maps a refused PUT /network/lan to its message; never parses error text. */
export function lanRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'invalid_request':
      return { error: m.lan_err_invalid() };
    case 'lan_subnet_overlap':
      return { error: m.lan_err_overlap() };
    case 'lan_dhcp_pool_outside':
      return { error: m.lan_err_pool() };
    case 'lan_dependents_outside':
      return { error: m.lan_err_dependents() };
    case 'lan_foreign_config':
      return { error: m.lan_err_foreign() };
    case 'lan_adguard_expert':
      return { error: m.lan_err_adguard_expert() };
    case 'lan_no_change':
      return { message: m.lan_no_change() };
    case 'uci_busy':
      return { error: m.dhcp_err_busy() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.lan_failed() };
  }
}

/** What the old-address page does with the PUT's fate. */
export type MoveOutcome =
  | { kind: 'moving'; seconds: number }
  | { kind: 'rolled_back' }
  | { kind: 'refused'; message?: string; error?: string };

/**
 * The PUT answers only while the old address still exists: a lost or late answer means the
 * change is probably applying, so the page sends the user to the new address either way.
 */
export function moveOutcome(
  result: 'lost' | { data?: TxnBody; error?: { code?: string } },
): MoveOutcome {
  if (result === 'lost') return { kind: 'moving', seconds: LOST_ANSWER_SECONDS };
  const t = result.data;
  if (t?.state === 'pending') return { kind: 'moving', seconds: t.window_seconds };
  if (t) return { kind: 'rolled_back' };
  return { kind: 'refused', ...lanRefusal(result.error?.code) };
}

/** The pending change as the transaction control adopts it on the new address. */
export function adoptedTxn(p: LanPending): TxnBody {
  return {
    id: p.id,
    state: 'pending',
    window_seconds: p.remaining_seconds,
    configs: ['network'],
    started: '',
    summary: '',
  };
}

export async function fetchLAN(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<LanView> {
  const { data, error, response } = await getLanAddress({ signal, client: requestClient });
  if (!data) {
    const code = (error as { code?: string } | undefined)?.code;
    throw new APIRequestError(
      code === 'lan_foreign_config' ? m.lan_err_foreign() : m.lan_unavailable(),
      response?.status,
    );
  }
  return data;
}
