import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { IPv6View } from '../api/gen';
import { ipv6Refusal } from '../api/ipv6';
import type { LoadStatus } from '../ui/loader.svelte';
import IPv6Card from './IPv6Card.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as IPv6View | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('IPv6 card', () => {
  it('offers the opposite of the current state', () => {
    loader.data = { enabled: true };
    const body = render(IPv6Card).body;
    expect(body).toContain('IPv6 on');
    expect(body).toContain('Turn IPv6 off');
  });

  it('maps refusals by code', () => {
    expect(ipv6Refusal('ipv6_vpn_leak').error).toContain('VPN');
    expect(ipv6Refusal('ipv6_no_change').message).toBeDefined();
  });
});
