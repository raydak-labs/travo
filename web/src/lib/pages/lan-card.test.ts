import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { LanView } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import LANAddressCard from './LANAddressCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as LanView | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('LAN address card', () => {
  it('shows the subnet and the change button', () => {
    loader.data = { address: '192.168.1.1/24', subnet: '192.168.1.0/24' };
    const body = render(LANAddressCard).body;
    expect(body).toContain('Current LAN subnet: 192.168.1.0/24.');
    expect(body).toContain('Change LAN address');
  });

  it('tells the new-address page a change waits for its confirm', () => {
    loader.data = {
      address: '192.168.8.1/24',
      subnet: '192.168.8.0/24',
      pending: { id: '0123456789abcdef', address: '192.168.8.1/24', remaining_seconds: 120 },
    };
    expect(render(LANAddressCard).body).toContain('The router moved to 192.168.8.1/24.');
  });
});
