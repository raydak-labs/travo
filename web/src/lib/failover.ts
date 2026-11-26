import './api/client';
import { client } from './api/client';
import { disableFailover, type FailoverWarning, getFailover, type TxnBody } from './api/gen';
import type { Client } from './api/gen/client';
import { APIRequestError } from './api/request';
import { m } from './paraglide/messages.js';

export type Uplink = 'wan' | 'wwan' | 'usbtether';
export const UPLINKS: Uplink[] = ['wan', 'wwan', 'usbtether'];

export async function fetchFailover(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getFailover({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.failover_unavailable(), response?.status);
  }
  return data;
}

/** Stops failover; the result is the message to show. */
export async function stopFailover(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<{ message: string } | { error: string }> {
  const { data, error } = await disableFailover({ signal, client: requestClient });
  if (data) return { message: m.failover_disabled() };
  return (error as { code?: string } | undefined)?.code === 'failover_busy'
    ? { error: m.failover_busy() }
    : { error: m.failover_disable_failed() };
}

const probeChecks: Record<NonNullable<TxnBody['reason']>, () => string> = {
  config: m.failover_check_config,
  service: m.failover_check_service,
  tracking: m.failover_check_tracking,
  hook: m.failover_check_hook,
  first_rule: m.failover_check_first_rule,
  policy_mark: m.failover_check_policy_mark,
  outbound: m.failover_check_outbound,
};

/** The rollback message, naming the probe check that failed when the daemon reports it. */
export function failoverRolledBack(result: Pick<TxnBody, 'reason' | 'detail'>): string {
  if (!result.reason) return m.failover_rolled_back();
  const why = probeChecks[result.reason]();
  return m.failover_rolled_back_reason({
    reason: result.detail ? `${why} (${result.detail})` : why,
  });
}

/** Maps a refused POST /failover to its message. */
export function failoverRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'failover_offline':
      return { error: m.failover_offline() };
    case 'failover_conflict':
      return { error: m.failover_conflict() };
    case 'failover_tailscale_conflict':
      return { error: m.failover_tailscale_conflict() };
    case 'failover_same_subnet':
      return { error: m.failover_same_subnet() };
    case 'failover_mangle_incompatible':
      return { error: m.failover_mangle_incompatible() };
    case 'validation':
      return { error: m.failover_invalid() };
    case 'failover_no_change':
      return { message: m.failover_no_change() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.failover_failed() };
  }
}

/** Adds or removes an uplink; a new one goes last (lowest priority). */
export function toggleUplink(list: Uplink[], name: Uplink): Uplink[] {
  return list.includes(name) ? list.filter((u) => u !== name) : [...list, name];
}

/** Moves the uplink at index one step up (-1) or down (+1); out of range is a no-op. */
export function moveUplink(list: Uplink[], index: number, step: -1 | 1): Uplink[] {
  const to = index + step;
  if (index < 0 || index >= list.length || to < 0 || to >= list.length) return list;
  const out = [...list];
  [out[index], out[to]] = [out[to] as Uplink, out[index] as Uplink];
  return out;
}

export function warningText(w: FailoverWarning): string {
  switch (w.reason) {
    case 'adopted':
      return m.failover_warning_adopted({ section: w.section });
    case 'operator_edited':
      return m.failover_warning_edited({ section: w.section });
    default:
      return m.failover_warning_stale({ section: w.section });
  }
}
