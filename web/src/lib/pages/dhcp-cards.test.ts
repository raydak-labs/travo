import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { DhcpView, DnsNamesOutputBody } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import DHCPServerCard from './DHCPServerCard.svelte';
import DNSNamesCard from './DNSNamesCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as DhcpView | DnsNamesOutputBody | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('DHCP server card', () => {
  it('shows the pool on the committed subnet', () => {
    loader.data = {
      enabled: true,
      start: 100,
      limit: 150,
      leasetime: '12h',
      subnet: '192.168.1.0/24',
      first: '192.168.1.100',
      last: '192.168.1.249',
      callerLease: true,
    };
    const body = render(DHCPServerCard).body;
    expect(body).toContain('Pool 192.168.1.100 – 192.168.1.249 in 192.168.1.0/24');
    expect(body).toContain('Save DHCP settings');
    expect(body).not.toContain('The DHCP server is off');
  });

  it('warns while the server is off', () => {
    loader.data = { enabled: false, start: 100, limit: 150, leasetime: '12h', callerLease: false };
    const body = render(DHCPServerCard).body;
    expect(body).toContain('The DHCP server is off');
    expect(body).not.toContain('Pool 192');
  });
});

describe('local DNS names card', () => {
  it('lists entries; only Travo entries are removable', () => {
    loader.data = {
      names: [
        { id: 'travo_dns_1', names: ['nas'], ip: '192.168.1.20', owned: true },
        { id: 'cfg0ab', names: ['router', 'gw'], ip: '192.168.1.1', owned: false },
      ],
    };
    const body = render(DNSNamesCard).body;
    expect(body).toContain('aria-label="Remove nas"');
    expect(body).not.toContain('aria-label="Remove router gw"');
    expect(body).toContain('Set in LuCI');
    expect(body).toContain('192.168.1.20');
  });

  it('empty: says so', () => {
    loader.data = { names: [] };
    expect(render(DNSNamesCard).body).toContain('No local DNS names yet.');
  });
});
