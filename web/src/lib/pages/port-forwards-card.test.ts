import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { PortForward } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import PortForwardsCard from './PortForwardsCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as PortForward[] | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const base = { src: 'wan', dest: 'lan', target: 'DNAT', enabled: true };

describe('port forwards card', () => {
  it('lists forwards; only Travo forwards are removable', () => {
    loader.data = [
      {
        ...base,
        id: 'travo_fwd_1',
        name: 'NAS',
        proto: 'both',
        external_port: '8080',
        target_ip: '192.168.1.50',
        target_port: '80',
        owned: true,
      },
      {
        ...base,
        id: 'cfg0ab',
        name: '',
        proto: 'tcp',
        external_port: '22001',
        target_ip: '',
        target_port: '22',
        owned: false,
        enabled: false,
      },
    ];
    const body = render(PortForwardsCard).body;
    expect(body).toContain('aria-label="Remove forward NAS"');
    expect(body).not.toContain('aria-label="Remove forward cfg0ab"');
    expect(body).toContain('both 8080 → 192.168.1.50:80');
    expect(body).toContain('tcp 22001 → router:22');
    expect(body).toContain('LuCI');
    expect(body).toContain('Off');
  });

  it('empty: says so and offers the form', () => {
    loader.data = [];
    const body = render(PortForwardsCard).body;
    expect(body).toContain('No port forwards.');
    expect(body).toContain('Add forward');
  });
});
