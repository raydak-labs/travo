import type { TxnBody } from './api/gen';
import { m } from './paraglide/messages.js';
import type { TxnPhase } from './ui/transaction-toast.svelte';

export type JoinState =
  | { state: 'idle' }
  | { state: 'joining'; ssid: string; ap?: boolean }
  | { state: 'failed'; code: string }
  | { state: TxnBody['state']; transaction: TxnBody; deadline: number };

export type JoinEvent =
  | { type: 'start'; ssid: string; ap?: boolean }
  | { type: 'transaction'; transaction: TxnBody; now: number }
  | { type: 'failure'; code: string };

export function reduceJoin(state: JoinState, event: JoinEvent): JoinState {
  switch (event.type) {
    case 'start':
      return event.ap
        ? { state: 'joining', ssid: event.ssid, ap: true }
        : { state: 'joining', ssid: event.ssid };
    case 'failure':
      return { state: 'failed', code: event.code };
    case 'transaction': {
      const { transaction } = event;
      const deadline =
        state.state === 'pending' && state.transaction.id === transaction.id
          ? state.deadline
          : event.now + transaction.window_seconds * 1000;
      return { state: transaction.state, transaction, deadline };
    }
  }
}

export function joinError(code: string) {
  switch (code) {
    case 'wifi_no_free_radio':
      return m.wifi_join_no_free_radio();
    case 'transaction_pending':
      return m.wifi_join_transaction_pending();
    case 'wifi_security_unsupported':
      return m.wifi_join_unsupported();
    case 'wifi_key_invalid':
    case 'wifi_key_required':
      return m.wifi_join_invalid_key();
    case 'wifi_network_not_found':
      return m.wifi_join_not_found();
    case 'wifi_lockout_risk':
      return m.wifi_join_lockout();
    case 'wifi_radio_in_use':
      return m.wifi_ap_radio_in_use();
    case 'wifi_ap_not_found':
      return m.wifi_ap_not_found();
    case 'wifi_guest_subnet_overlap':
      return m.wifi_guest_overlap();
    case 'wifi_guest_conflict':
      return m.wifi_guest_conflict();
    case 'wifi_mac_invalid':
      return m.wifi_mac_invalid();
    case 'wifi_mac_clone_unavailable':
      return m.wifi_mac_clone_unavailable();
    case 'wifi_no_uplink':
      return m.wifi_mac_no_uplink();
    case 'wifi_ssid_invalid':
      return m.wifi_ssid_invalid();
    case 'wifi_no_change':
      return m.wifi_join_no_change();
    default:
      return m.wifi_join_unavailable();
  }
}

/** What the transaction toast shows for a join or AP change; `settling` while confirm/rollback runs. */
export function joinPhase(state: JoinState, settling: boolean): TxnPhase {
  switch (state.state) {
    case 'idle':
      return { phase: 'idle' };
    case 'joining':
      return {
        phase: 'applying',
        text: state.ap
          ? m.wifi_ap_applying({ ssid: state.ssid })
          : m.wifi_join_connecting({ ssid: state.ssid }),
      };
    case 'pending':
      return { phase: 'pending', deadline: state.deadline, settling };
    case 'restoring':
      return { phase: 'restoring', text: m.txn_restoring() };
    case 'confirmed':
      return { phase: 'done', tone: 'success', text: m.wifi_join_confirmed() };
    case 'rolled_back':
      return { phase: 'done', tone: 'info', text: m.wifi_join_rolled_back() };
    case 'rollback_failed':
      return { phase: 'done', tone: 'error', text: m.wifi_join_rollback_failed() };
    case 'failed':
      return { phase: 'done', tone: 'error', text: joinError(state.code) };
  }
}
