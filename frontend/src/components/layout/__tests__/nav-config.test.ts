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

  it('uses Connect / Internet & LAN / Apps / Extras / Tools labels', () => {
    const wifi = NAV_ENTRIES.find((e) => e.id === 'wifi');
    const network = NAV_ENTRIES.find((e) => e.id === 'network');
    const services = NAV_ENTRIES.find((e) => e.id === 'services');
    expect(wifi?.kind === 'group' && wifi.items.map((i) => i.label)).toEqual([
      'Connect',
      'Extras',
    ]);
    expect(network?.kind === 'group' && network.items.map((i) => i.label)).toEqual([
      'Status',
      'Internet & LAN',
      'Tools',
    ]);
    expect(services?.kind === 'group' && services.items[0]?.label).toBe('Apps');
  });

  it('defaults all groups collapsed when storage empty', () => {
    expect(loadSidebarGroupState()).toEqual({
      wifi: false,
      network: false,
      services: false,
      system: false,
    });
  });

  it('ignores legacy otg-sidebar-groups key', () => {
    localStorage.setItem(
      'otg-sidebar-groups',
      JSON.stringify({ wifi: true, network: true, services: true, system: true }),
    );
    expect(loadSidebarGroupState().wifi).toBe(false);
  });

  it('persists under v2 key', () => {
    saveSidebarGroupState('wifi', true);
    expect(localStorage.getItem('otg-sidebar-groups-v2')).toContain('"wifi":true');
  });
});
