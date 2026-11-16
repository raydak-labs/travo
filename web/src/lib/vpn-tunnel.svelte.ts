import './api/client';
import {
  confirmTransaction,
  getTransaction,
  rollbackTransaction,
  setVpnTunnel,
  type TunnelStatus,
  type TxnBody,
} from './api/gen';
import { m } from './paraglide/messages.js';

/** Enabled means the tunnel is configured on, whether or not it is up yet. */
export function tunnelEnabled(state: TunnelStatus['state']): boolean {
  return state !== 'disabled' && state !== 'configured';
}

/** Maps a refused POST /vpn/tunnel to the message the VPN page and dashboard share. */
export function tunnelRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'vpn_profile_required':
      return { error: m.vpn_tunnel_profile_required() };
    case 'vpn_no_change':
      return { message: m.vpn_tunnel_no_change() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.vpn_tunnel_failed() };
  }
}

/**
 * One guarded WireGuard toggle: request, poll the transaction, confirm or roll back.
 * `refresh` reloads authoritative tunnel status after every settled outcome.
 */
export function createTunnelControl(refresh: () => unknown) {
  const request = new AbortController();
  let applying = $state(false);
  let settling = $state(false);
  let transaction = $state<TxnBody>();
  let message = $state('');
  let error = $state('');
  let statusPoll: AbortController | undefined;
  // Polling starts with the first request: no transaction can be open before it.
  let timer: ReturnType<typeof setInterval> | undefined;
  const open = $derived(transaction?.state === 'pending' || transaction?.state === 'restoring');
  const busy = $derived(applying || settling || open);

  function accept(result: TxnBody) {
    transaction = result;
    if (result.state === 'confirmed') message = m.vpn_tunnel_confirmed();
    if (result.state === 'rolled_back') message = m.vpn_tunnel_rolled_back();
    if (result.state === 'rollback_failed') error = m.vpn_tunnel_rollback_failed();
    if (result.state !== 'pending' && result.state !== 'restoring') void refresh();
  }

  async function set(enabled: boolean) {
    if (busy || request.signal.aborted) return;
    timer ??= setInterval(() => void poll(), 2000);
    applying = true;
    transaction = undefined;
    message = '';
    error = '';
    try {
      const response = await setVpnTunnel({ body: { enabled }, signal: request.signal });
      if (request.signal.aborted) return;
      if (response.data) accept(response.data);
      else {
        const refusal = tunnelRefusal(response.error?.code);
        message = refusal.message ?? '';
        error = refusal.error ?? '';
        void refresh();
      }
    } catch {
      if (!request.signal.aborted) error = m.vpn_tunnel_failed();
    } finally {
      applying = false;
    }
  }

  async function poll() {
    if (statusPoll || settling || !open || !transaction) return;
    const id = transaction.id;
    const current = new AbortController();
    statusPoll = current;
    const deadline = setTimeout(() => current.abort(), 4000);
    try {
      const response = await getTransaction({ path: { id }, signal: current.signal });
      if (current.signal.aborted || statusPoll !== current || transaction?.id !== id) return;
      if (response.data) accept(response.data);
      else if (response.response?.status === 404) {
        transaction = undefined;
        error = m.vpn_tunnel_unknown_result();
        void refresh();
      }
    } catch {
      // A failed status read gives no evidence about the transaction outcome.
    } finally {
      clearTimeout(deadline);
      if (statusPoll === current) statusPoll = undefined;
    }
  }

  async function settle(confirm: boolean) {
    if (settling || transaction?.state !== 'pending') return;
    settling = true;
    statusPoll?.abort();
    statusPoll = undefined;
    try {
      const operation = confirm ? confirmTransaction : rollbackTransaction;
      const response = await operation({ path: { id: transaction.id }, signal: request.signal });
      if (request.signal.aborted) return;
      if (response.data) accept(response.data);
      else if (response.error?.code === 'restoring' || response.error?.code === 'rollback_failed') {
        accept({ ...transaction, state: response.error.code });
      }
    } catch {
      // Resume polling after a failed command.
    } finally {
      settling = false;
    }
  }

  function stop() {
    clearInterval(timer);
    statusPoll?.abort();
    request.abort();
  }

  return {
    get applying() {
      return applying;
    },
    get settling() {
      return settling;
    },
    get transaction() {
      return transaction;
    },
    get message() {
      return message;
    },
    get error() {
      return error;
    },
    get busy() {
      return busy;
    },
    set,
    settle,
    poll,
    stop,
  };
}

export type TunnelControl = ReturnType<typeof createTunnelControl>;

export const tunnelStates = {
  disabled: { kind: 'neutral', label: m.vpn_state_disabled },
  configured: { kind: 'info', label: m.vpn_state_configured },
  enabled_not_up: { kind: 'warn', label: m.vpn_state_enabled_not_up },
  no_handshake: { kind: 'warn', label: m.vpn_state_no_handshake },
  connected: { kind: 'ok', label: m.vpn_state_connected },
} as const;
