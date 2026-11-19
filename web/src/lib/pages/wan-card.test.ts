import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { WanView } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import WANCard from './WANCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as WanView | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('WAN card', () => {
  it('shows the protocol choice and the port', () => {
    loader.data = {
      proto: 'dhcp',
      ipaddr: '',
      gateway: '',
      dns: [],
      username: '',
      password_set: false,
      device: 'wan',
    };
    const body = render(WANCard).body;
    expect(body).toContain('DHCP (automatic)');
    expect(body).toContain('PPPoE');
    expect(body).toContain('Runs on port wan.');
    expect(body).toContain('Apply WAN settings');
  });
});
