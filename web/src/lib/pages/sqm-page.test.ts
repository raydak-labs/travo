import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchSQM } from '../api/sqm';
import type { LoadStatus } from '../ui/loader.svelte';
import SQM from './SQM.svelte';

const ready: LoadStatus = { state: 'ready' };
const { createLoader } = vi.hoisted(() => ({ createLoader: vi.fn() }));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const loader = (data: unknown) => ({ status: ready, data, retry: vi.fn(), stop: vi.fn() });

describe('SQM page', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders essential settings form and secondary LuCI link', () => {
    vi.stubGlobal('window', { location: { protocol: 'http:', hostname: '192.168.1.1' } });
    createLoader.mockImplementation((fn: unknown) =>
      fn === fetchSQM
        ? loader({
            enabled: false,
            interface: 'eth1',
            download: 85000,
            upload: 10000,
            qdisc: 'cake',
            script: 'piece_of_cake.qos',
            linklayer: 'none',
            overhead: 0,
          })
        : loader({
            partial: false,
            interfaces: [{ name: 'wan', device: 'eth1', uplink: true }],
          }),
    );
    const body = render(SQM).body;
    vi.unstubAllGlobals();

    expect(body).toContain('SQM');
    expect(body).toContain('Queue settings');
    expect(body).toContain('Enable SQM');
    expect(body).toContain('Download speed (kbit/s)');
    expect(body).toContain('Upload speed (kbit/s)');
    expect(body).toContain('Queue discipline');
    expect(body).toContain('Queue setup script');
    expect(body).toContain('Link layer adaptation');
    expect(body).toContain('Per-packet overhead (bytes)');
    expect(body).toContain('Save settings');
    expect(body).toMatch(/href="[^"]*admin\/network\/sqm"/);
  });
});
