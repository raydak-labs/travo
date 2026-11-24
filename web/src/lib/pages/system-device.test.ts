import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';

const info = {
  hostname: 'OpenWrt',
  model: 'GL.iNet GL-AXT1800',
  board_name: 'glinet,gl-axt1800',
  openwrt: '25.12.6',
  revision: 'r33311-981c978eff',
  target: 'qualcommax/ipq60xx',
  kernel: '6.12.112',
  uptime_seconds: 224,
  load: [0.39, 0.24, 0.1] as [number, number, number],
  memory: { total_bytes: 428646400, available_bytes: 149295104 },
  storage: { total_bytes: 81616896, available_bytes: 69513216 },
  app: { version: 'dev', build_time: '' },
};

describe('System Device card', () => {
  it('renders the fixture values once the loader is ready', async () => {
    vi.doMock('../theme.svelte', () => ({ theme: { value: 'light', toggle: () => {} } }));
    // A server render never runs effects, so hand the page a loader that is already ready.
    vi.doMock('../ui/loader.svelte', () => ({
      createLoader: () => ({ status: { state: 'ready' }, data: info, retry: async () => {} }),
    }));
    // Lazy cards have their own tests; here the host would get the system info as a component.
    vi.doMock('./PageHost.svelte', () => ({ default: () => {} }));
    const { default: System } = await import('./System.svelte');
    const { body } = render(System);
    vi.unstubAllGlobals();
    vi.doUnmock('../ui/loader.svelte');
    vi.doUnmock('./PageHost.svelte');
    for (const text of [
      'Device',
      'Hardware',
      'GL.iNet GL-AXT1800',
      'OpenWrt',
      '25.12.6',
      'r33311-981c978eff',
      '6.12.112',
      '3m',
      '0.39 · 0.24 · 0.10',
      '266.4 MB of 408.8 MB',
      '11.5 MB of 77.8 MB',
      'dev',
      '—',
    ]) {
      expect(body).toContain(text);
    }
  });
});
