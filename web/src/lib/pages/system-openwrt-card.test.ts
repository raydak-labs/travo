import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { OpenWrtCheck } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemOpenWrtCard from './SystemOpenWrtCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as OpenWrtCheck | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const checked: OpenWrtCheck = {
  checked_at: '2026-11-18T10:00:00Z',
  target: 'qualcommax/ipq60xx',
  current: '25.12.3',
  in_series: '25.12.6',
  next_series: '',
  package_manager: 'apk',
  lists_refreshed: false,
  packages_read: true,
  packages: [{ name: 'dnsmasq', installed: '2.93-r1', available: '2.93-r2' }],
};

describe('OpenWrt updates card', () => {
  it('shows the last result, the packages and the LuCI link', () => {
    loader.data = checked;
    const body = render(SystemOpenWrtCard).body;
    expect(body).toContain('25.12.6');
    expect(body).toContain('1 packages can be upgraded');
    expect(body).toContain('dnsmasq');
    expect(body).toContain('Open LuCI attended sysupgrade');
    expect(body).toContain('Check now');
  });

  it('says when no check ran yet', () => {
    loader.data = { ...checked, checked_at: null, packages: [] };
    expect(render(SystemOpenWrtCard).body).toContain('Not since the service started');
  });
});
