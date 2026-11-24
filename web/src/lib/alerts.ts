import type { Alert, AlertList } from './api/gen';
import { m } from './paraglide/messages.js';
import type { Route } from './router';

/** The shell banner shows danger and info alerts; warnings stay on their pages. */
export function bannerAlerts(list: AlertList | undefined): Alert[] {
  return (list?.alerts ?? []).filter((a) => a.severity !== 'warning');
}

/** The page that owns the condition. A guard belongs to the feature its name starts with. */
export function alertRoute(alert: Alert): Route {
  switch (alert.code) {
    case 'adguard_fail_open':
      return 'services';
    case 'uplink_reconnect_exhausted':
      return 'wifi';
    case 'data_budget_warning':
    case 'data_budget_exceeded':
      return 'internet';
    case 'firmware_upgrade_completed':
      return 'system-updates';
    case 'stale_guard': {
      const name = alert.subject ?? '';
      if (/^(wifi|wireless|uplink)/.test(name)) return 'wifi';
      if (/^(vpn|wireguard)/.test(name)) return 'vpn';
      if (/^(adguard|tailscale)/.test(name)) return 'services';
      if (/^(failover|usbtether)/.test(name)) return 'internet';
      if (/^ddns/.test(name)) return 'network-ipv6';
      if (/^interface/.test(name)) return 'network-lan';
      if (/^(firmware|update)/.test(name)) return 'system-updates';
      if (/^factory-reset/.test(name)) return 'system-reset';
      return 'system';
    }
    default:
      return 'system';
  }
}

/** Text by stable code; the API never sends prose. */
export function alertMessage(alert: Alert): string {
  switch (alert.code) {
    case 'stale_guard':
      return m.alert_stale_guard({ name: alert.subject ?? '' });
    case 'overlay_low':
      return m.alert_overlay_low();
    case 'persistence_degraded':
      return m.alert_persistence_degraded();
    case 'adguard_fail_open':
      return m.alert_adguard_fail_open();
    case 'uplink_reconnect_exhausted':
      return m.alert_uplink_reconnect_exhausted();
    case 'data_budget_warning':
      return m.alert_data_budget_warning();
    case 'data_budget_exceeded':
      return m.alert_data_budget_exceeded();
    case 'firmware_upgrade_completed':
      return m.alert_firmware_upgrade_completed();
    case 'memory_low':
      return m.alert_memory_low();
  }
}
