import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardConnectivity } from '../api/gen';
import { fetchHealth } from '../api/health';
import type { LoadStatus } from '../ui/loader.svelte';
import Dashboard from './Dashboard.svelte';

const { createLoader, dashboardEvents } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  dashboardEvents: {
    connectivity: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
    resources: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    throughput: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    tailscale: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
    uplinks: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    clients: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

type Snapshot = Omit<DashboardConnectivity, 'captive' | 'medium'> & {
  medium?: DashboardConnectivity['medium'];
  captive?: Partial<DashboardConnectivity['captive']>;
};

function dashboard(status: LoadStatus, data: Snapshot | undefined) {
  createLoader.mockReturnValue({
    status: { state: 'ready' },
    data: undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  });
  dashboardEvents.connectivity.status = status;
  dashboardEvents.connectivity.data = data && {
    medium: null,
    ...data,
    captive: {
      dns_bypass: false,
      pending: false,
      restore_in_seconds: null,
      dhcp_renew: 'not_applicable',
      ...data.captive,
    },
  };
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body;
}

describe('Dashboard connectivity', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('shows active uplink and link up even when the internet is offline', () => {
    const body = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        medium: 'wifi',
        carrier: true,
        internet: 'offline',
        portal_url: null,
      },
    );
    expect(body).toContain('phy0-sta0');
    expect(body).toContain('· WiFi');
    expect(body).toContain('Link up');
    expect(body).toContain('Offline');
    expect(createLoader).toHaveBeenCalledWith(fetchHealth);
  });

  it('names a USB tether uplink by its medium', () => {
    const body = dashboard(
      { state: 'ready' },
      {
        active_interface: 'eth0',
        medium: 'usb',
        carrier: true,
        internet: 'online',
        portal_url: null,
      },
    );
    expect(body).toContain('eth0');
    expect(body).toContain('· USB tethering');
  });

  it('distinguishes unknown and down carrier', () => {
    const data = {
      active_interface: null,
      carrier: null,
      internet: 'online',
      portal_url: null,
    } as const;
    const body = dashboard({ state: 'ready' }, data);
    expect(body).toContain('Unknown');
    expect(body).toContain('Online');
    expect(body).not.toContain('Link down');
    expect(dashboard({ state: 'ready' }, { ...data, carrier: false })).toContain('Link down');
  });

  it('retains the last connectivity snapshot on failed refresh', () => {
    const body = dashboard(
      { state: 'error', message: 'Connectivity unavailable' },
      {
        active_interface: 'phy0-sta0',
        medium: 'wifi',
        carrier: true,
        internet: 'captive',
        portal_url: null,
      },
    );
    expect(body).toContain('Last known value');
    expect(body).toContain('phy0-sta0');
    expect(body).toContain('Captive portal');
    expect(body).toContain('Connectivity unavailable');
  });

  it('keeps the WiFi button available when connectivity is unavailable', () => {
    const body = dashboard({ state: 'error', message: 'Connectivity unavailable' }, undefined);

    expect(body).toContain('Connect to WiFi');
    expect(body.replace(/<!--.*?-->/g, '')).toMatch(
      /<a[^>]*href="#\/wifi"[^>]*>Connect to WiFi<\/a>/,
    );
  });

  it('shows no hotel login card unless a portal is detected', () => {
    const body = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        medium: 'wifi',
        carrier: true,
        internet: 'online',
        portal_url: null,
      },
    );
    expect(body).not.toContain('Hotel WiFi login');
    expect(body).not.toContain('Open hotel login');
  });

  it('links the detected portal in a new tab', () => {
    const body = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        carrier: true,
        internet: 'captive',
        portal_url: 'http://portal.example/login?a=1&b=2',
      },
    ).replace(/<!--.*?-->/g, '');
    expect(body).toContain('Hotel WiFi login');
    const link = body.match(/<a[^>]*>Open hotel login<\/a>/)?.[0] ?? '';
    expect(link).toContain('href="http://portal.example/login?a=1&amp;b=2"');
    expect(link).toContain('target="_blank"');
    expect(link).toContain('rel="noopener noreferrer"');
  });

  it('offers the automatic login only while a portal is detected', () => {
    const snapshot = {
      active_interface: 'phy0-sta0',
      carrier: true,
      internet: 'captive',
      portal_url: 'http://portal.example/login',
    } as const;
    expect(dashboard({ state: 'ready' }, snapshot)).toContain('Try automatic login');
    const offline = dashboard(
      { state: 'ready' },
      { ...snapshot, internet: 'offline', portal_url: null },
    );
    expect(offline).toContain('Hotel WiFi login');
    expect(offline).not.toContain('Try automatic login');
  });

  it('offers the DNS bypass when connected but offline, and hides it when online', () => {
    const offline = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        medium: 'wifi',
        carrier: true,
        internet: 'offline',
        portal_url: null,
      },
    );
    expect(offline).toContain('Hotel WiFi login');
    expect(offline).toContain('Use hotel DNS for 5 min');
    expect(offline).not.toContain('Open hotel login');
    const noLink = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        medium: 'wifi',
        carrier: false,
        internet: 'offline',
        portal_url: null,
      },
    );
    expect(noLink).not.toContain('Hotel WiFi login');
  });

  it('shows the active bypass with its countdown and a restore action', () => {
    const body = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        carrier: true,
        internet: 'online',
        portal_url: null,
        captive: { dns_bypass: true, restore_in_seconds: 125 },
      },
    );
    expect(body).toContain('Hotel DNS in use');
    expect(body).toContain('Restores automatically in 2:05.');
    expect(body).toContain('Restore DNS now');
    expect(body).not.toContain('Use hotel DNS for 5 min');
  });

  it('keeps an unfinished DNS change visible without offering another change', () => {
    const body = dashboard(
      { state: 'ready' },
      {
        active_interface: 'phy0-sta0',
        carrier: true,
        internet: 'online',
        portal_url: null,
        captive: { pending: true },
      },
    );
    expect(body).toContain('A DNS change did not finish');
    expect(body).toContain('Retry DNS restore');
    expect(body).not.toContain('Use hotel DNS for 5 min');
    expect(body).not.toContain('Restore DNS now');
  });

  it('offers the wwan renew only when the daemon says it applies', () => {
    const snapshot = {
      active_interface: 'phy0-sta0',
      carrier: true,
      internet: 'captive',
      portal_url: null,
    } as const;
    expect(dashboard({ state: 'ready' }, snapshot)).not.toContain('Renew hotel connection');
    expect(
      dashboard({ state: 'ready' }, { ...snapshot, captive: { dhcp_renew: 'available' } }),
    ).toContain('Renew hotel connection');
    const blocked = dashboard(
      { state: 'ready' },
      { ...snapshot, internet: 'online', captive: { dhcp_renew: 'blocked' } },
    );
    expect(blocked).toContain('An earlier renew could not prove');
    expect(blocked).toContain('Retry renew');
  });
});
