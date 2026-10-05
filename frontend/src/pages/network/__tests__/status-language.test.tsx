import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { API_ROUTES, type FailoverConfig } from '@shared/index';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/mocks/server';
import { statusDotClass, statusDotIdleClass } from '@/lib/status-dot';
import { NetworkPageAdvancedPanel } from '../network-page-advanced-panel';
import { FailoverCard } from '../failover-card';
import { UptimeLogCard } from '../uptime-log-card';
import { WanStatusCard } from '../wan-status-card';
import { DoHCard } from '../doh-card';
import { DataUsageUsageBar } from '../data-usage-usage-bar';

/**
 * The status language is one vocabulary: a semantic token per tone, whether it
 * reaches the screen as a pill, a dot or a bar. These assertions exist so a new
 * `text-emerald-700` cannot quietly become a second answer for "online".
 */

function failoverWith(tracking_state: FailoverConfig['candidates'][number]['tracking_state']) {
  return {
    available: true,
    service_installed: true,
    enabled: true,
    active_interface: 'wan',
    candidates: [
      {
        id: 'wan',
        label: 'WAN (Ethernet)',
        interface_name: 'wan',
        kind: 'ethernet',
        available: true,
        enabled: true,
        priority: 1,
        tracking_state,
        is_up: tracking_state === 'online',
      },
    ],
    health: {
      track_ips: ['1.1.1.1'],
      reliability: 2,
      count: 3,
      timeout: 3,
      interval: 60,
      failure_interval: 10,
      recovery_interval: 60,
      down: 3,
      up: 2,
    },
  } satisfies FailoverConfig;
}

describe('status dot colours', () => {
  it('come from the semantic tokens rather than a palette literal', () => {
    expect(statusDotClass(true)).toContain('var(--status-ok-border)');
    expect(statusDotClass(false)).toContain('var(--status-danger-border)');
    expect(statusDotIdleClass).toContain('var(--status-neutral-border)');
  });
});

describe('NetworkPageAdvancedPanel', () => {
  it('labels its group with the shared section heading', async () => {
    renderWithProviders(<NetworkPageAdvancedPanel />);

    const heading = await screen.findByRole('heading', { level: 2, name: 'Power tools' });
    expect(heading.className).toContain('uppercase');
  });
});

describe('FailoverCard status', () => {
  it('states the operator state as a pill with the tone token', async () => {
    server.use(
      http.get(API_ROUTES.network.failover, () => HttpResponse.json(failoverWith('online'))),
    );
    renderWithProviders(<FailoverCard />);

    const enabled = await screen.findByText('Enabled');
    expect(enabled.className).toContain('var(--status-ok-border)');
  });

  it('states an offline candidate row with the danger token', async () => {
    server.use(
      http.get(API_ROUTES.network.failover, () => HttpResponse.json(failoverWith('offline'))),
    );
    renderWithProviders(<FailoverCard />);

    const [offline] = await screen.findAllByText('Offline');
    expect(offline.className).toContain('var(--status-danger-border)');
  });
});

describe('UptimeLogCard', () => {
  it('names the current state with a pill and tones the rows by state', async () => {
    const { container } = renderWithProviders(<UptimeLogCard />);

    const connected = await screen.findByText('Connected now');
    expect(connected.className).toContain('var(--status-ok-border)');

    await waitFor(() => {
      expect(screen.getAllByText('Disconnected').length).toBeGreaterThan(0);
    });
    const disconnectedRow = screen.getAllByText('Disconnected')[0];
    expect(disconnectedRow.className).toContain('var(--status-danger-text)');
    expect(container.innerHTML).toContain('var(--status-danger-border)');
  });
});

describe('WanStatusCard', () => {
  it('draws the active uplink dot from the shared status dot tokens', async () => {
    const { container } = renderWithProviders(<WanStatusCard />);

    await screen.findByText('Uplink Connected');
    const dot = container.querySelector('span.rounded-full');
    expect(dot?.className).toContain('var(--status-ok-border)');
  });
});

describe('DoHCard', () => {
  it('states the resolver state as a pill instead of plain copy', async () => {
    renderWithProviders(<DoHCard />);

    const disabled = await screen.findByText('Disabled');
    expect(disabled.className).toContain('var(--status-neutral-border)');
  });
});

describe('DataUsageUsageBar', () => {
  it('tones the bar and the over-budget note from the semantic tokens', () => {
    const { container } = render(
      <DataUsageUsageBar used={1500} limit={1000} label="Monthly budget" />,
    );

    expect(container.innerHTML).toContain('var(--status-danger-border)');
    expect(screen.getByText(/Over budget by/).className).toContain('var(--status-danger-text)');
  });

  it('warns before the budget is spent', () => {
    const { container } = render(
      <DataUsageUsageBar used={850} limit={1000} label="Monthly budget" />,
    );

    expect(container.innerHTML).toContain('var(--status-warn-border)');
  });
});
