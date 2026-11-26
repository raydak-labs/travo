import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import type { DashboardUplinks } from '../api/gen';
import { topologyModel } from '../topology';
import { connectivity, throughput, uplinks } from '../topology.test-inputs';
import type { LoadStatus } from '../ui/loader.svelte';
import UplinkCards from './UplinkCards.svelte';

const ready: LoadStatus = { state: 'ready' };

// The mock daemon's uplinks, with the #337 fields as the route test asserts them.
const filled: DashboardUplinks = uplinks;
const empty: DashboardUplinks = {
  ...uplinks,
  uplinks: uplinks.uplinks.map((u) => ({
    ...u,
    proto: null,
    gateway: null,
    band: null,
    signal_dbm: null,
  })),
};

function cards(data: DashboardUplinks): string {
  const model = topologyModel({ uplinks: data, connectivity, throughput });
  return render(UplinkCards, {
    props: {
      sources: model.sources,
      uplinks: { status: ready, data, retry: () => {} },
      status: { connectivity: ready, throughput: ready },
    },
  }).body.replace(/<!--[^>]*-->/g, '');
}
const card = (body: string, title: string) => {
  const start = body.indexOf(`${title}</h2>`);
  return body.slice(start, body.indexOf('</dl>', start));
};

describe('UplinkCards details', () => {
  it('shows the WAN protocol, and protocol only on WAN', () => {
    const body = cards(filled);
    expect(card(body, 'Ethernet (wan)')).toContain('Protocol');
    expect(card(body, 'Ethernet (wan)')).toContain('DHCP (automatic)');
    expect(card(body, 'WiFi uplink (wwan)')).not.toContain('Protocol');
  });

  it('shows band, signal and gateway on the WiFi uplink', () => {
    const wwan = card(cards(filled), 'WiFi uplink (wwan)');
    expect(wwan).toContain('5 GHz');
    expect(wwan).toContain('-63 dBm');
    expect(wwan).toContain('192.168.90.2');
  });

  it('omits each row whose field is null', () => {
    const body = cards(empty);
    for (const label of ['Protocol', 'Band', 'Signal', 'Gateway'])
      expect(body).not.toContain(`>${label}<`);
  });
});
