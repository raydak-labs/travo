import { describe, it, expect, beforeEach } from 'vitest';
import {
  NAV_ENTRIES,
  loadSidebarGroupState,
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
      'Advanced',
    ]);
    expect(network?.kind === 'group' && network.defaultTo).toBe('/network');
    expect(network?.kind === 'group' && network.items.map((i) => i.label)).toEqual([
      'Status',
      'Internet & LAN',
      'Advanced',
    ]);
    expect(services?.kind === 'group' && services.defaultTo).toBe('/services');
    expect(services?.kind === 'group' && services.items[0]?.label).toBe('Apps');
    expect(system?.kind === 'group' && system.defaultTo).toBe('/system');
  });

  it('defaults all groups collapsed when storage empty', () => {
    expect(loadSidebarGroupState()).toEqual({
      wifi: false,
      network: false,
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
