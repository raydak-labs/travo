import { describe, it, expect } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/mocks/server';
import { WifiWirelessPanel } from '../wifi-wireless-panel';

function band() {
  return screen.getByRole('region', { name: 'WiFi status' });
}

const TILE_TITLES = ['Connection', 'Internet', 'Mode'];

// The band exists so the first glance of the page answers "is my Wi-Fi
// working" without scrolling; these lock the three facts and their wording.
describe('WiFi summary band', () => {
  it('renders the three status tiles with a connected, reachable state', async () => {
    renderWithProviders(<WifiWirelessPanel />);

    await waitFor(() => {
      expect(within(band()).getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });

    for (const title of TILE_TITLES) {
      expect(within(band()).getByText(title)).toBeInTheDocument();
    }
    expect(within(band()).getByText(/-42 dBm · 82%/)).toBeInTheDocument();
    expect(within(band()).getByText('5 GHz · channel 36')).toBeInTheDocument();
    expect(within(band()).getByText('Reachable')).toBeInTheDocument();
    expect(within(band()).getByText('Client (STA)')).toBeInTheDocument();
    // Radio configuration and health live on WiFi / Advanced and in the health
    // banner; repeating them here pushed the controls below the fold.
    expect(within(band()).queryByText('Radios')).not.toBeInTheDocument();
    expect(within(band()).queryByText('Health')).not.toBeInTheDocument();
  });

  it('reports the failure in one tile without blanking the others', async () => {
    server.use(
      http.get(API_ROUTES.network.status, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );
    renderWithProviders(<WifiWirelessPanel />);

    await waitFor(() => {
      expect(within(band()).getByRole('alert')).toHaveTextContent('ubus busy');
    });
    // A failed reachability query is not a reachability fact: no "Not reachable".
    expect(within(band()).queryByText('Not reachable')).not.toBeInTheDocument();
    expect(within(band()).getByText('Connection')).toBeInTheDocument();
    expect(within(band()).getByText('Hotel_Guest_5G')).toBeInTheDocument();
  });

  it('is always visible, with nothing to expand or collapse', async () => {
    renderWithProviders(<WifiWirelessPanel />);

    await waitFor(() => {
      expect(within(band()).getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });

    const region = band();
    expect(within(region).queryAllByRole('button')).toHaveLength(0);
    expect(region.querySelector('[aria-expanded]')).toBeNull();
    for (const title of TILE_TITLES) {
      expect(within(region).getByText(title)).toBeVisible();
    }
  });
});
