import { describe, it, expect, beforeEach } from 'vitest';
import {
  NAV_ENTRIES,
  flattenNavRoutes,
  loadSidebarGroupState,
  navEntriesForPages,
  saveSidebarGroupState,
} from '@/components/layout/nav-config';

describe('nav-config traveler IA', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('uses Connect / Advanced / Internet & LAN labels and daily defaults', () => {
    const wifi = NAV_ENTRIES.find((e) => e.id === 'wifi');
    const network = NAV_ENTRIES.find((e) => e.id === 'network');
    const services = NAV_ENTRIES.find((e) => e.id === 'services');
    const system = NAV_ENTRIES.find((e) => e.id === 'system');
    expect(wifi?.kind === 'group' && wifi.defaultTo).toBe('/wifi');
    expect(wifi?.kind === 'group' && wifi.items.map((i) => i.label)).toEqual([
      'Connect',
      // Qualified so the two "Advanced" pages are distinguishable in the nav.
      'WiFi / Advanced',
    ]);
    expect(network?.kind === 'group' && network.defaultTo).toBe('/network');
    expect(network?.kind === 'group' && network.items.map((i) => i.label)).toEqual([
      'Status',
      'Internet & LAN',
      'Network / Advanced',
    ]);
    expect(services?.kind === 'group' && services.defaultTo).toBe('/services');
    expect(services?.kind === 'group' && services.items[0]?.label).toBe('Apps');
    expect(system?.kind === 'group' && system.defaultTo).toBe('/system');
  });

  it('defaults Network open when storage empty', () => {
    expect(loadSidebarGroupState()).toEqual({
      wifi: false,
      network: true,
      services: false,
      system: false,
    });
  });

  it('ignores legacy otg-sidebar-groups keys', () => {
    localStorage.setItem(
      'otg-sidebar-groups',
      JSON.stringify({ wifi: true, network: true, services: true, system: true }),
    );
    localStorage.setItem(
      'otg-sidebar-groups-v2',
      JSON.stringify({ wifi: true, network: true, services: true, system: true }),
    );
    expect(loadSidebarGroupState().wifi).toBe(false);
  });

  it('persists under v3 key', () => {
    saveSidebarGroupState('wifi', true);
    expect(localStorage.getItem('otg-sidebar-groups-v3')).toContain('"wifi":true');
  });
});

describe('optional service pages', () => {
  const allPresent = { tailscale: true, speedtest: true, sqm: true };

  it('orders Apps, Tailscale, SQM, Speedtest when every optional page is available', () => {
    const services = navEntriesForPages(allPresent).find((e) => e.id === 'services');
    expect(services?.kind === 'group' && services.items.map((i) => i.label)).toEqual([
      'Apps',
      'Tailscale',
      'SQM',
      'Speedtest',
    ]);
  });

  it('leaves only Apps when no optional package is installed', () => {
    const services = navEntriesForPages({
      tailscale: false,
      speedtest: false,
      sqm: false,
    }).find((e) => e.id === 'services');
    expect(services?.kind === 'group' && services.items.map((i) => i.to)).toEqual(['/services']);
  });

  it('keeps the collapsed rail in step with the expanded group', () => {
    const rail = flattenNavRoutes(allPresent);
    expect(rail.map((r) => r.to)).toContain('/services/sqm');
    expect(
      flattenNavRoutes({ tailscale: false, speedtest: false, sqm: false }).map((r) => r.to),
    ).not.toContain('/services/speedtest');
  });
});
