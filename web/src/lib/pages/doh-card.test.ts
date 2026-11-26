import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import { dohError, validDoH } from '../api/doh';
import type { DoHView } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import DoHCard from './DoHCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as DoHView | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('DoH card', () => {
  it('shows whether the upstream is encrypted', () => {
    loader.data = { upstreams: ['https://dns10.quad9.net/dns-query'], encrypted: true };
    const body = render(DoHCard).body;
    expect(body).toContain('Encrypted upstream');
    expect(body).toContain('Cloudflare');
  });

  it('validates like the daemon', () => {
    expect(validDoH('https://dns.google/dns-query')).toBe(true);
    for (const bad of [
      'http://dns.google/q',
      'https://u:p@dns.google/q',
      'https://dns.google/q?x=1',
      'tls://a',
      '',
    ]) {
      expect(validDoH(bad)).toBe(false);
    }
    expect(dohError('doh_upstream_unreachable')).toContain('nothing was changed');
  });
});
