import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { DeviceSettings } from '../api/system-settings';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemSettingsCard from './SystemSettingsCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as DeviceSettings | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('Device settings card', () => {
  it('offers the zone list and the NTP fields', () => {
    loader.data = {
      settings: {
        hostname: 'OpenWrt',
        zonename: 'UTC',
        timezone: 'GMT0',
        ntp: { enabled: true, servers: ['0.openwrt.pool.ntp.org'] },
      },
      zonenames: ['UTC', 'Europe/Berlin'],
    };
    const body = render(SystemSettingsCard).body;
    expect(body).toContain('Device settings');
    expect(body).toContain('list="travo-zonenames"');
    expect(body).toContain('<option value="Europe/Berlin">');
    expect(body).toContain('Time servers');
    expect(body).toContain('Save settings');
  });

  it('shows a load error with retry instead of the form', () => {
    loader.data = undefined;
    loader.status = { state: 'error', message: 'Device settings are unavailable' };
    const body = render(SystemSettingsCard).body;
    expect(body).toContain('Device settings are unavailable');
    expect(body).not.toContain('Save settings');
  });
});
