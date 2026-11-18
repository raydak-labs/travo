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
  it('banners only danger alerts', () => {
    const list = {
      alerts: [
        alert({ id: 'a', code: 'adguard_fail_open', severity: 'danger' }),
        alert({ id: 'b', code: 'overlay_low' }),
      ],
    };
    expect(bannerAlerts(list).map((a) => a.id)).toEqual(['a']);
    expect(bannerAlerts(undefined)).toEqual([]);
  });

  it('links each alert to its owning page', () => {
    expect(alertRoute(alert({ code: 'adguard_fail_open' }))).toBe('services');
    expect(alertRoute(alert({ code: 'uplink_reconnect_exhausted' }))).toBe('wifi');
    expect(alertRoute(alert({ code: 'persistence_degraded' }))).toBe('system');
    const guard = (subject: string) => alertRoute(alert({ code: 'stale_guard', subject }));
    expect(guard('wifi-apply-in-progress')).toBe('wifi');
    expect(guard('vpn-tunnel-in-progress')).toBe('vpn');
    expect(guard('tailscale-change-in-progress')).toBe('services');
    expect(guard('firmware-upgrade-in-progress')).toBe('system');
  });

  it('names the guard in its message', () => {
    expect(
      alertMessage(alert({ code: 'stale_guard', subject: 'wifi-apply-in-progress' })),
    ).toContain('wifi-apply-in-progress');
  });
});
