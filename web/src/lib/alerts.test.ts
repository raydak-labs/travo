import { describe, expect, it } from 'vitest';
import { alertMessage, alertRoute, bannerAlerts } from './alerts';
import type { Alert } from './api/gen';

const alert = (over: Partial<Alert>): Alert => ({
  id: 'x',
  code: 'overlay_low',
  severity: 'warning',
  since: null,
  ...over,
});

describe('alerts', () => {
  it('banners danger and info alerts, never warnings', () => {
    const list = {
      alerts: [
        alert({ id: 'a', code: 'adguard_fail_open', severity: 'danger' }),
        alert({ id: 'b', code: 'overlay_low' }),
        alert({ id: 'c', code: 'firmware_upgrade_completed', severity: 'info' }),
      ],
    };
    expect(bannerAlerts(list).map((a) => a.id)).toEqual(['a', 'c']);
    expect(alertRoute(alert({ code: 'firmware_upgrade_completed' }))).toBe('system-updates');
    expect(alertMessage(alert({ code: 'firmware_upgrade_completed' }))).toContain(
      'upgrade finished',
    );
    expect(alertRoute(alert({ code: 'memory_low', severity: 'danger' }))).toBe('system');
    expect(alertMessage(alert({ code: 'memory_low' }))).toContain('low on memory');
    expect(bannerAlerts(undefined)).toEqual([]);
  });

  it('links each alert to its owning page', () => {
    expect(alertRoute(alert({ code: 'adguard_fail_open' }))).toBe('services');
    expect(alertRoute(alert({ code: 'uplink_reconnect_exhausted' }))).toBe('wifi');
    expect(alertRoute(alert({ code: 'persistence_degraded' }))).toBe('system');
    expect(alertRoute(alert({ code: 'data_budget_exceeded' }))).toBe('internet');
    expect(alertMessage(alert({ code: 'data_budget_warning' }))).toContain('almost used up');
    const guard = (subject: string) => alertRoute(alert({ code: 'stale_guard', subject }));
    expect(guard('wifi-apply-in-progress')).toBe('wifi');
    expect(guard('vpn-tunnel-in-progress')).toBe('vpn');
    expect(guard('tailscale-change-in-progress')).toBe('services');
    expect(guard('firmware-upgrade-in-progress')).toBe('system-updates');
    expect(guard('update-in-progress')).toBe('system-updates');
    expect(guard('factory-reset-in-progress')).toBe('system-reset');
    expect(guard('failover-install-in-progress')).toBe('internet');
    expect(guard('usbtether-install-in-progress')).toBe('internet');
    expect(guard('ddns-install-in-progress')).toBe('network-ipv6');
    expect(guard('interface-restart-in-progress')).toBe('network-lan');
    expect(guard('reboot-in-progress')).toBe('system');
  });

  it('names the guard in its message', () => {
    expect(
      alertMessage(alert({ code: 'stale_guard', subject: 'wifi-apply-in-progress' })),
    ).toContain('wifi-apply-in-progress');
  });
});
