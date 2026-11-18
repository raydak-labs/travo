import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { type ClientList, fetchClients, type Reservation } from '../api/clients';
import type { LoadStatus } from '../ui/loader.svelte';
import Clients from './Clients.svelte';

const { createLoader, clients, reservations } = vi.hoisted(() => {
  const loader = <T>() => ({
    status: { state: 'ready' } as LoadStatus,
    data: undefined as T | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  });
  return {
    createLoader: vi.fn(),
    clients: loader<ClientList>(),
    reservations: loader<Reservation[]>(),
  };
});
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

function page(list: ClientList, reserved: Reservation[] = []) {
  clients.data = list;
  reservations.data = reserved;
  return render(Clients).body;
}

describe('Clients page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockImplementation((fn: unknown) =>
      fn === fetchClients ? clients : reservations,
    );
  });

  it('polls the client list and loads reservations once', () => {
    page({ clients: [], partial: false });
    expect(createLoader).toHaveBeenCalledWith(fetchClients, 10000);
    expect(createLoader.mock.calls.filter((call) => call[1] === undefined)).toHaveLength(1);
  });

  it('shows names, connection badges and actions per client', () => {
    const html = page(
      {
        partial: false,
        clients: [
          {
            mac: '9c:eb:e8:d3:f8:d1',
            ip: '192.168.1.2',
            connection: 'lan',
            reserved: false,
            blocked: false,
          },
          {
            mac: '22:4e:76:6c:2d:62',
            ip: '192.168.1.151',
            hostname: 'phone',
            connection: 'wifi',
            ssid: 'TravelAP',
            signal: -38,
            reserved: true,
            blocked: true,
          },
        ],
      },
      [{ mac: '22:4e:76:6c:2d:62', ip: '192.168.1.151', owned: true }],
    );
    expect(html).toContain('phone');
    expect(html).toContain('WiFi · -38 dBm');
    expect(html).toContain('on TravelAP');
    expect(html).toContain('Reserved IP');
    expect(html).toContain('Unblock');
    expect(html.match(/Disconnect/g)).toHaveLength(1); // only the WiFi station
    expect(html).toContain('Remove the reservation for phone');
  });

  it('says when the list is partial or empty', () => {
    const html = page({ clients: [], partial: true });
    expect(html).toContain('the list may be incomplete');
    expect(html).toContain('No devices seen yet.');
  });
});
