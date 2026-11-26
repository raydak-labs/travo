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
import type { TxnPhase } from './ui/transaction-toast.svelte';

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

/** Splits the routes textarea into prefixes: one per line or comma, blanks dropped. */
export function parseRoutes(text: string): string[] {
  return text
    .split(/[\s,]+/)
    .map((route) => route.trim())
    .filter((route) => route !== '');
}

/** Maps a refused POST /vpn/split to its message. */
export function splitRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'validation':
      return { error: m.vpn_split_invalid() };
    case 'vpn_profile_required':
      return { error: m.vpn_split_profile_required() };
    case 'vpn_no_change':
      return { message: m.vpn_split_no_change() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.vpn_split_failed() };
  }
}

/** Maps a refused POST /vpn/killswitch to its message. */
export function killSwitchRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'vpn_not_connected':
      return { error: m.vpn_kill_not_connected() };
    case 'vpn_killswitch_unsupported':
      return { error: m.vpn_kill_unsupported() };
    case 'vpn_no_change':
      return { message: m.vpn_kill_no_change() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.vpn_kill_failed() };
  }
}

type Submit = (
  enabled: boolean,
  signal: AbortSignal,
) => Promise<{ data?: TxnBody; error?: { code?: string } }>;

export type ToggleOptions = {
  submit: Submit;
  refusal: (code: string | undefined) => { message?: string; error?: string };
  confirmed: () => string;
  /** Receives the settled answer: a feature may name why its probe rolled back. */
  rolledBack: (result: TxnBody) => string;
  rollbackFailed: () => string;
  /** Request failed without an answer; defaults to the tunnel message. */
  failed?: () => string;
  /** The transaction vanished before it settled; defaults to the tunnel message. */
  unknown?: () => string;
};

const tunnelToggle: ToggleOptions = {
  submit: (enabled, signal) => setVpnTunnel({ body: { enabled }, signal }),
  refusal: tunnelRefusal,
  confirmed: m.vpn_tunnel_confirmed,
  rolledBack: m.vpn_tunnel_rolled_back,
  rollbackFailed: m.vpn_tunnel_rollback_failed,
};

/**
 * One guarded toggle (WireGuard tunnel by default): request, poll the transaction, confirm or
 * roll back. `refresh` reloads authoritative status after every settled outcome.
 */
export function createTunnelControl(refresh: () => unknown, options: ToggleOptions = tunnelToggle) {
  const request = new AbortController();
  let applying = $state(false);
  let settling = $state(false);
  let transaction = $state<TxnBody>();
  // performance.now() time the confirm window closes; set when a new transaction arrives.
  let deadline = $state(0);
  let message = $state('');
  let error = $state('');
  let statusPoll: AbortController | undefined;
  // Polling starts with the first request: no transaction can be open before it.
  let timer: ReturnType<typeof setInterval> | undefined;
  const open = $derived(transaction?.state === 'pending' || transaction?.state === 'restoring');
  const busy = $derived(applying || settling || open);

  function accept(result: TxnBody) {
    if (result.state === 'pending' && transaction?.id !== result.id) {
      deadline = performance.now() + result.window_seconds * 1000;
    }
    transaction = result;
    if (result.state === 'confirmed') message = options.confirmed();
    if (result.state === 'rolled_back') message = options.rolledBack(result);
    if (result.state === 'rollback_failed') error = options.rollbackFailed();
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
      const response = await options.submit(enabled, request.signal);
      if (request.signal.aborted) return;
      if (response.data) accept(response.data);
      else {
        const refusal = options.refusal(response.error?.code);
        message = refusal.message ?? '';
        error = refusal.error ?? '';
        void refresh();
      }
    } catch {
      if (!request.signal.aborted) error = (options.failed ?? m.vpn_tunnel_failed)();
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
        error = (options.unknown ?? m.vpn_tunnel_unknown_result)();
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
    get deadline() {
      return deadline;
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

type ControlView = Pick<
  TunnelControl,
  'applying' | 'settling' | 'transaction' | 'deadline' | 'message' | 'error'
>;

/** What the transaction toast shows for a guarded toggle; `applying` names the running step. */
export function controlPhase(control: ControlView, applying: string): TxnPhase {
  const state = control.transaction?.state;
  if (control.applying) return { phase: 'applying', text: applying };
  if (state === 'pending') {
    return { phase: 'pending', deadline: control.deadline, settling: control.settling };
  }
  if (state === 'restoring') return { phase: 'restoring', text: m.txn_restoring() };
  if (control.error) return { phase: 'done', tone: 'error', text: control.error };
  if (control.message) {
    return {
      phase: 'done',
      tone: state === 'confirmed' ? 'success' : 'info',
      text: control.message,
    };
  }
  return { phase: 'idle' };
}

export const tunnelStates = {
  disabled: { kind: 'neutral', label: m.vpn_state_disabled },
  configured: { kind: 'info', label: m.vpn_state_configured },
  enabled_not_up: { kind: 'warn', label: m.vpn_state_enabled_not_up },
  no_handshake: { kind: 'warn', label: m.vpn_state_no_handshake },
  connected: { kind: 'ok', label: m.vpn_state_connected },
} as const;
