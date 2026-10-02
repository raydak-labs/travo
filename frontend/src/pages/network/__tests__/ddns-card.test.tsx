import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { mockDDNSConfig } from '@/mocks/data';
import { server } from '@/mocks/server';
import { DdnsCard } from '../ddns-card';

function renderCard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <DdnsCard />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

function mockDDNS(available: boolean) {
  server.use(
    http.get(API_ROUTES.network.ddns, () =>
      HttpResponse.json({ config: mockDDNSConfig, available }),
    ),
  );
}

// ddns-scripts is not installed on a stock router, and it is not in the service
// catalog, so the UI cannot offer an install. The backend answers 503 to every
// write in that state, so the card must say so up front rather than present a
// form whose only possible outcome is an error.
describe('DdnsCard when ddns-scripts is missing', () => {
  it('explains that DDNS is unavailable instead of showing the form', async () => {
    mockDDNS(false);
    renderCard();

    await waitFor(() => {
      expect(screen.getByText(/Dynamic DNS is unavailable/)).toBeInTheDocument();
    });
    // The message names the package to install, which is the only actionable
    // part: ddns is not in the service catalog, so there is no install button.
    expect(screen.getByText(/opkg install ddns-scripts/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Save DDNS Settings/ })).not.toBeInTheDocument();
  });
});

describe('DdnsCard when ddns-scripts is installed', () => {
  it('shows the editable form', async () => {
    mockDDNS(true);
    renderCard();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Save DDNS Settings/ })).toBeInTheDocument();
    });
    expect(screen.queryByText(/Dynamic DNS is unavailable/)).not.toBeInTheDocument();
  });
});
