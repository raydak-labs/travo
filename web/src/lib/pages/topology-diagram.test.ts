import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import { type TopologyModel, topologyModel } from '../topology';
import {
  clients,
  connectivity,
  resources,
  tailscale,
  throughput,
  tunnel,
  uplinks,
} from '../topology.test-inputs';
import type { LoadStatus } from '../ui/loader.svelte';
import TopologyDiagram from './TopologyDiagram.svelte';

const ready: LoadStatus = { state: 'ready' };
type Part = 'uplinks' | 'connectivity' | 'throughput' | 'clients';
const allReady: Record<Part, LoadStatus> & { vpn?: LoadStatus } = {
  uplinks: ready,
  connectivity: ready,
  throughput: ready,
  clients: ready,
};
const full = topologyModel({
  uplinks,
  connectivity,
  throughput,
  clients,
  resources,
  tailscale,
  vpn: tunnel('connected'),
});

function html(
  model: TopologyModel,
  status: Partial<Record<Part | 'vpn', LoadStatus>> = {},
): string {
  return render(TopologyDiagram, {
    props: { model, status: { ...allReady, ...status } },
  }).body.replace(/<!--[^>]*-->/g, '');
}

const sourceItem = (body: string, state: string) =>
  body.match(new RegExp(`<li[^>]*data-state="${state}"[^>]*>[\\s\\S]*?</li>`))?.[0] ?? '';

describe('TopologyDiagram', () => {
  it('renders sources, router and clients in that DOM order', () => {
    const body = html(full);
    const sources = body.indexOf('data-part="sources"');
    const router = body.indexOf('data-part="router"');
    const clientsAt = body.indexOf('data-part="clients"');
    expect(sources).toBeGreaterThan(-1);
    expect(sources).toBeLessThan(router);
    expect(router).toBeLessThan(clientsAt);
  });

  it('is a named figure with aria-hidden connectors', () => {
    const body = html(full);
    expect(body).toMatch(/<figure[^>]*aria-labelledby="([^"]+)"/);
    const id = body.match(/<figure[^>]*aria-labelledby="([^"]+)"/)?.[1];
    expect(body).toContain(`<figcaption id="${id}" class="sr-only">`);
    for (const svg of body.match(/<svg[^>]*>/g) ?? []) {
      // Lucide icons carry aria-hidden too; every svg in the diagram is decorative.
      expect(svg).toContain('aria-hidden="true"');
    }
  });

  it('renders each source state with its text and connector class', () => {
    const body = html(full);
    expect(sourceItem(body, 'active')).toContain('Active');
    expect(sourceItem(body, 'active')).toContain('href="#/wifi"');
    expect(sourceItem(body, 'down')).toContain('Down');
    expect(sourceItem(body, 'down')).toContain('href="#/network/wan"');
    expect(sourceItem(body, 'not_configured')).toContain('Not set up');
    expect(sourceItem(body, 'not_configured')).toContain('href="#/internet"');
    expect(body).toContain('topology-link topology-link-active');
    expect(body).toContain('topology-link topology-link-down');
    // A source that is not set up gets no connector:
    // usbtether is not set up, so it has no link: two source links plus the two client links.
    expect(body.match(/<path d="M /g)).toHaveLength(4);
  });

  it('draws standby dashed and unknown with text', () => {
    const standby = topologyModel({
      uplinks,
      connectivity: { ...connectivity, active_interface: null },
    });
    expect(html(standby)).toContain('topology-link-standby');
    expect(sourceItem(html(standby), 'standby')).toContain('Standby');
    const unknown = html(topologyModel({ clients }));
    expect(unknown).toContain('topology-link-unknown');
    expect(sourceItem(unknown, 'unknown')).toContain('Unknown');
  });

  it('shows rates only for active or standby sources with data', () => {
    const body = html(full);
    expect(sourceItem(body, 'active')).toContain('data-rate');
    expect(sourceItem(body, 'active')).toContain('10 kbit/s');
    expect(sourceItem(body, 'down')).not.toContain('data-rate');
    expect(sourceItem(body, 'not_configured')).not.toContain('data-rate');
  });

  it('links client counts to the clients page and marks partial data', () => {
    const body = html(full);
    const part = body.slice(body.indexOf('data-part="clients"'));
    expect(part.match(/<a href="#\/clients"/g)).toHaveLength(2);
    expect(part).toContain('1 on WiFi');
    expect(part).toContain('1 wired');
    expect(body).not.toContain('Incomplete');
    const partial = topologyModel({ clients: { ...clients, partial: true } });
    expect(html(partial)).toContain('Incomplete');
  });

  it('renders unknown clients as unknown, never 0', () => {
    const body = html(topologyModel({ uplinks, connectivity }));
    const part = body.slice(body.indexOf('data-part="clients"'));
    expect(part).toContain('Unknown');
    expect(part).not.toMatch(/>0<\/span>/);
  });

  it('shows the router badges and hides Tailscale when absent', () => {
    const body = html(full);
    expect(body).toContain('OpenWrt');
    expect(body).toContain('GL.iNet GL-AXT1800');
    const badges = body.slice(body.indexOf('data-part="router-badges"'));
    // Colour carries the state; the state text is there for screen readers and the tooltip.
    expect(badges).toMatch(/title="Internet: Online"[^>]*data-kind="ok"/);
    expect(badges).toContain('<span class="sr-only">: Online</span>');
    expect(body).not.toContain('Tailscale');
    // IPv6 shows only once its setting is known.
    expect(badges).not.toContain('IPv6');
    const withTailscale = topologyModel({
      tailscale: { ...tailscale, installed: true, running: true, backend_state: 'Running' },
    });
    expect(html(withTailscale)).toContain('href="#/vpn/tailscale"');
  });

  it('shows IPv6 green when on and grey when off', () => {
    const on = render(TopologyDiagram, {
      props: { model: full, status: allReady, ipv6: true },
    }).body;
    expect(on).toMatch(/title="IPv6: On"[^>]*data-kind="ok"/);
    const off = render(TopologyDiagram, {
      props: { model: full, status: allReady, ipv6: false },
    }).body;
    expect(off).toMatch(/title="IPv6: Off"[^>]*data-kind="neutral"/);
  });

  it('lists every part in the hidden summary', () => {
    const body = html(full);
    const summary = body.match(/<ul class="sr-only"[^>]*>([\s\S]*?)<\/ul>/)?.[1] ?? '';
    expect(summary).toContain('Ethernet (wan): Down');
    expect(summary).toContain('WiFi uplink (wwan): Active, HomeUplink, 10 kbit/s down, Unknown up');
    expect(summary).toContain('USB tethering: Not set up');
    expect(summary).toContain('OpenWrt (GL.iNet GL-AXT1800): Internet: Online, VPN: Connected');
    expect(summary).toContain('Clients: 1 on WiFi, 1 wired');
  });

  it('dims and marks a stale part, keeping its values', () => {
    const body = html(full, { clients: { state: 'error', message: 'Clients unavailable' } });
    const part = body.slice(body.indexOf('data-part="clients"'));
    expect(part).toContain('opacity-60');
    expect(part).toContain('status-stale');
    expect(part).toContain('1 on WiFi');
    expect(body.slice(0, body.indexOf('data-part="router"'))).not.toContain('status-stale');
  });

  it('dims only the rates when just throughput fails', () => {
    const body = html(full, { throughput: { state: 'error', message: 'Throughput unavailable' } });
    const active = sourceItem(body, 'active');
    expect(active).not.toContain('status-stale');
    expect(active).toMatch(/opacity-60" data-rate/);
  });

  it('marks the router stale when the VPN poll fails with a known state', () => {
    const body = html(full, { vpn: { state: 'error', message: 'VPN status unavailable' } });
    const router = body.slice(
      body.indexOf('data-part="router"'),
      body.indexOf('data-part="clients"'),
    );
    expect(router).toContain('status-stale');
    expect(router).toContain('Connected');
  });

  it('shows a skeleton until the first data arrives', () => {
    const loading: LoadStatus = { state: 'loading' };
    const body = html(topologyModel({}), { uplinks: loading, clients: loading });
    expect(body).toContain('data-slot="skeleton"');
    expect(body).not.toContain('<figure');
  });

  it('uses no arbitrary pixel text sizes', async () => {
    const source = (await import('./TopologyDiagram.svelte?raw')).default as string;
    expect(source).not.toMatch(/text-\[/);
  });
});
