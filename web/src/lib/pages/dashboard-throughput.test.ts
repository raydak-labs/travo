import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Rate } from '../api/gen';
import { tailscale } from '../topology.test-inputs';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
import Dashboard from './Dashboard.svelte';

const { createLoader, dashboardEvents } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  dashboardEvents: {
    connectivity: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    resources: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    throughput: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
    tailscale: {
      status: { state: 'ready' } as LoadStatus,
      data: undefined as unknown,
      retry: vi.fn(),
    },
    uplinks: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    clients: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    alerts: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

function dashboard(status: LoadStatus, interfaces?: Rate[]) {
  createLoader.mockReturnValue({
    status: { state: 'ready' },
    data: undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  });
  dashboardEvents.throughput.status = status;
  dashboardEvents.throughput.data = interfaces ? { interfaces } : undefined;
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body;
}

const interfaces = [
  { name: 'br-lan', rx_bytes_per_second: 0, tx_bytes_per_second: null },
  { name: 'phy0-sta0', rx_bytes_per_second: 1024, tx_bytes_per_second: 256 },
] satisfies Rate[];

describe('Dashboard throughput', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('keeps interface traffic behind a closed toggle', () => {
    const body = dashboard({ state: 'ready' }, []);
    expect(body).toContain('Show interfaces');
    expect(body).toMatch(/aria-expanded="false"/);
  });

  it('lists named interfaces and distinguishes unknown rates from zero', () => {
    const body = dashboard({ state: 'ready' }, interfaces);
    expect(body).toContain('br-lan');
    expect(body).toContain('phy0-sta0');
    expect(body).toContain('0 B/s');
    expect(body).toContain('Unknown');
    expect(body).toContain('1.0 KB/s');
    expect(body).toContain('256 B/s');
  });

  it('keeps last values visible when refresh fails', () => {
    const body = dashboard({ state: 'error', message: 'Throughput unavailable' }, interfaces);
    expect(body).toContain('br-lan');
    expect(body).toContain('Last known value');
    expect(body).toContain('Throughput unavailable');
  });

  it('shows an empty successful snapshot without a failure', () => {
    const body = dashboard({ state: 'ready' }, []);
    expect(body).toContain('No interfaces reported');
    expect(body).not.toContain('Last known value');
  });
});

describe('Dashboard Tailscale', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
  });
  afterEach(() => {
    dashboardEvents.tailscale.data = undefined;
    vi.unstubAllGlobals();
  });

  it('stays hidden while the package is absent', () => {
    dashboardEvents.tailscale.data = { installed: false };
    expect(dashboard({ state: 'ready' }, [])).not.toContain('#/vpn/tailscale');
  });

  it('shows the pushed state on the map and links the Tailscale page', () => {
    dashboardEvents.tailscale.data = {
      installed: true,
      running: true,
      backend_state: 'Running',
      self: { ips: ['100.64.0.1'] },
      settings: { exit_node: 'nA', advertise_lan: false, ssh: false },
    };
    const body = dashboard({ state: 'ready' }, []);
    expect(body).toContain('Tailscale: Connected');
    const plain = body.replace(/<!--[^>]*-->/g, '');
    expect(plain).toMatch(
      /<a href="#\/vpn\/tailscale"[^>]*title="Tailscale: Connected"[^>]*data-kind="ok"/,
    );
  });
});

describe('Dashboard Quick status Tailscale row', () => {
  const installed = { ...tailscale, installed: true, version: '1.104.1' };
  const connected = { ...installed, running: true, backend_state: 'Running' };
  const quick = (body: string) => {
    const plain = body.replace(/<!--[^>]*-->/g, '');
    const start = plain.indexOf('Quick status');
    return plain.slice(start, plain.indexOf('aria-labelledby', start + 1));
  };
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
    dashboardEvents.tailscale.status = { state: 'ready' };
  });
  afterEach(() => {
    dashboardEvents.tailscale.data = undefined;
    dashboardEvents.tailscale.status = { state: 'ready' };
    vi.unstubAllGlobals();
  });

  it('links the Tailscale page with its state and an on switch when connected', () => {
    dashboardEvents.tailscale.data = connected;
    const part = quick(dashboard({ state: 'ready' }, []));
    expect(part).toMatch(/<a href="#\/vpn\/tailscale"[^>]*>Tailscale<\/a>/);
    expect(part).toContain('Connected');
    expect(switchState(part, 'dashboard-tailscale-switch')).toEqual({
      checked: true,
      disabled: false,
    });
  });

  it('shows an off switch when installed but stopped', () => {
    dashboardEvents.tailscale.data = installed;
    const part = quick(dashboard({ state: 'ready' }, []));
    expect(part).toContain('Off');
    expect(switchState(part, 'dashboard-tailscale-switch')).toEqual({
      checked: false,
      disabled: false,
    });
  });

  it('disables the switch while busy or stale', () => {
    dashboardEvents.tailscale.data = { ...connected, busy: true };
    expect(
      switchState(quick(dashboard({ state: 'ready' }, [])), 'dashboard-tailscale-switch'),
    ).toEqual({ checked: true, disabled: true });
    dashboardEvents.tailscale.data = connected;
    dashboardEvents.tailscale.status = { state: 'error', message: 'Tailscale unavailable' };
    const part = quick(dashboard({ state: 'ready' }, []));
    expect(part).toContain('Last known value');
    expect(switchState(part, 'dashboard-tailscale-switch')).toEqual({
      checked: true,
      disabled: true,
    });
  });

  it('hides the row when the package is absent', () => {
    dashboardEvents.tailscale.data = tailscale;
    const part = quick(dashboard({ state: 'ready' }, []));
    expect(part).not.toContain('#/vpn/tailscale');
    expect(part).not.toContain('dashboard-tailscale-switch');
  });

  it('keeps the row with the error and Retry when the first read fails', () => {
    dashboardEvents.tailscale.status = { state: 'error', message: 'Tailscale unavailable' };
    const part = quick(dashboard({ state: 'ready' }, []));
    expect(part).toContain('#/vpn/tailscale');
    expect(part).toContain('Tailscale unavailable');
    expect(part).toContain('Retry');
  });
});
