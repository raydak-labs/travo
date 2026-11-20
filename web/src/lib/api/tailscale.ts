import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  getTailscale,
  installTailscale,
  loginTailscale,
  logoutTailscale,
  setTailscaleSettings,
  type TailscaleSettings,
  type TailscaleStatus,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchTailscale(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getTailscale({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.tailscale_unavailable(), response?.status);
  }
  return data;
}

export type TailscaleAction = 'install' | 'login' | 'logout' | 'settings';

/** What the Tailscale page shows for one tailscaled install, run and backend state. */
export type TailscalePhase = 'absent' | 'stopped' | 'login' | 'connected' | 'off' | 'starting';

export function tailscalePhase(
  st: Pick<TailscaleStatus, 'installed' | 'running' | 'backend_state'>,
): TailscalePhase {
  if (!st.installed) return 'absent';
  if (!st.running) return 'stopped';
  switch (st.backend_state) {
    case 'Running':
      return 'connected';
    case 'NeedsLogin':
    case 'NeedsMachineAuth':
      return 'login';
    case 'Stopped':
    case 'NoState':
      return 'off';
    default:
      return 'starting';
  }
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function tailscaleError(code: string | undefined): string {
  switch (code) {
    case 'tailscale_offline':
      return m.tailscale_err_offline();
    case 'tailscale_no_space':
      return m.tailscale_err_no_space();
    case 'tailscale_failover_conflict':
      return m.tailscale_err_failover();
    case 'tailscale_busy':
      return m.tailscale_err_busy();
    case 'tailscale_not_running':
      return m.tailscale_err_not_running();
    case 'tailscale_exit_node_unknown':
      return m.tailscale_err_exit_unknown();
    case 'tailscale_exit_node_conflict':
      return m.tailscale_err_exit_conflict();
    case 'tailscale_exit_node_offline':
      return m.tailscale_err_exit_offline();
    case 'tailscale_no_lan':
      return m.tailscale_err_no_lan();
    default:
      return m.tailscale_err_failed();
  }
}

/** Runs one change; resolves to the new status, or an error message. */
export async function changeTailscale(
  action: TailscaleAction,
  signal?: AbortSignal,
  settings?: TailscaleSettings,
): Promise<{ status: TailscaleStatus } | { error: string }> {
  try {
    const response =
      action === 'install'
        ? await installTailscale({ signal })
        : action === 'login'
          ? await loginTailscale({ signal })
          : action === 'logout'
            ? await logoutTailscale({ signal })
            : await setTailscaleSettings({
                signal,
                body: settings ?? { exit_node: '', advertise_lan: false, ssh: false },
              });
    if (response.data) return { status: response.data };
    return { error: tailscaleError((response.error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: m.tailscale_err_failed() };
  }
}

/** Refusals of POST /tailscale/firewall for the shared transaction control. */
export function firewallRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'tailscale_no_change':
      return { message: m.tailscale_fw_no_change() };
    case 'tailscale_firewall_conflict':
      return { error: m.tailscale_fw_conflict() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.tailscale_fw_failed() };
  }
}
