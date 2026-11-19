import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import { ddnsError, ddnsRequest } from '../api/ddns';
import type { DdnsView } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import DDNSCard from './DDNSCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as DdnsView | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('DDNS card', () => {
  const base: DdnsView = {
    installed: true,
    enabled: true,
    service: 'duckdns.org',
    domain: 'trip.duckdns.org',
    username: '',
    password_set: true,
    registered_ip: '203.0.113.7',
    services: ['duckdns.org', 'dynv6.com'],
  };

  it('offers the install when the package is absent', () => {
    loader.data = { ...base, installed: false, services: [] };
    expect(render(DDNSCard).body).toContain('Install ddns-scripts');
  });

  it('shows the providers and never a stored token', () => {
    loader.data = base;
    const body = render(DDNSCard).body;
    expect(body).toContain('dynv6.com');
    expect(body).toContain('Registered address 203.0.113.7');
    expect(body).toContain('Stored; leave empty to keep');
  });

  it('builds the strict body', () => {
    expect(
      ddnsRequest({
        enabled: true,
        service: 'duckdns.org',
        domain: ' a.duckdns.org ',
        username: ' ',
        password: '',
      }),
    ).toEqual({ enabled: true, service: 'duckdns.org', domain: 'a.duckdns.org' });
    expect(ddnsError('ddns_not_installed')).toContain('not installed');
  });
});
