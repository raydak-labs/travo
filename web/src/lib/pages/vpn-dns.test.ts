import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DnsStatus } from '../api/gen';
import { fetchVpnDns } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import VPNDNSCard from './VPNDNSRow.svelte';

const { createLoader, dns } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  dns: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as DnsStatus | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

function card(data: DnsStatus | undefined, status: LoadStatus = { state: 'ready' }) {
  dns.data = data;
  dns.status = status;
  return render(VPNDNSCard).body;
}

const active: DnsStatus = {
  layer: 'active',
  resolvers: ['10.64.0.1'],
  effective: ['10.64.0.1'],
  top: 'vpn',
};

describe('VPN DNS card', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockReturnValue(dns);
  });

  it('polls the DNS status', () => {
    card(active);
    expect(createLoader).toHaveBeenCalledWith(fetchVpnDns, 10000);
  });

  it('shows the active layer and its resolvers', () => {
    const body = card(active);
    expect(body).toContain('Through the tunnel');
    expect(body).toContain('10.64.0.1');
    expect(body).toContain('Verify DNS');
    expect(body).not.toContain('Overridden');
  });

  it('names a higher layer that overrides the tunnel resolvers', () => {
    expect(card({ ...active, top: 'adguard' })).toContain('Overridden by adguard');
  });

  it('shows inactive and failed layers', () => {
    const inactive = card({ layer: 'inactive', resolvers: [], effective: [], top: null });
    expect(inactive).toContain('Uplink DNS');
    expect(inactive).toContain('None in profile');
    expect(card({ ...active, layer: 'failed' })).toContain('DNS change did not finish');
  });

  it('shows unavailable status', () => {
    const body = card(undefined, { state: 'error', message: 'VPN DNS status unavailable' });
    expect(body).toContain('VPN DNS status unavailable');
  });
});
