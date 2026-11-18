import type { AcceptResult, DashboardConnectivity } from './api/gen';
import { m } from './paraglide/messages.js';

/** The captive card shows only when it offers an action (invariants §12). */
export function captiveCardVisible(info: DashboardConnectivity): boolean {
  return (
    info.internet === 'captive' ||
    info.captive.dns_bypass ||
    info.captive.pending ||
    info.captive.dhcp_renew === 'blocked' ||
    (info.carrier === true && info.internet === 'offline')
  );
}

/** m:ss, for the daemon's relative restore countdown (clients never compare clocks). */
export function formatCountdown(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export function captiveError(code: string | undefined): string {
  switch (code) {
    case 'captive_no_uplink':
      return m.captive_error_no_uplink();
    case 'captive_no_uplink_dns':
      return m.captive_error_no_uplink_dns();
    case 'dns_change_pending':
      return m.captive_error_pending();
    case 'dns_foreign_staged':
      return m.captive_error_foreign_staged();
    default:
      return m.captive_error_failed();
  }
}

export function renewError(code: string | undefined): string {
  switch (code) {
    case 'captive_renew_not_applicable':
      return m.captive_error_renew_not_applicable();
    case 'captive_renew_busy':
      return m.captive_error_renew_busy();
    case 'captive_renew_unproven':
      return m.captive_error_renew_unproven();
    default:
      return m.captive_error_renew_failed();
  }
}

/** How the card reports a finished automatic login: every outcome is an expected answer. */
export function acceptMessage(outcome: AcceptResult['outcome']): {
  kind: 'success' | 'info' | 'error';
  text: string;
} {
  switch (outcome) {
    case 'online':
      return { kind: 'success', text: m.captive_accept_online() };
    case 'needs_user':
      return { kind: 'info', text: m.captive_accept_needs_user() };
    case 'no_form':
      return { kind: 'error', text: m.captive_accept_no_form() };
    default:
      return { kind: 'error', text: m.captive_accept_failed() };
  }
}

export function acceptError(code: string | undefined): string {
  switch (code) {
    case 'captive_accept_not_captive':
      return m.captive_error_accept_not_captive();
    case 'captive_accept_busy':
      return m.captive_error_accept_busy();
    default:
      return m.captive_accept_failed();
  }
}
