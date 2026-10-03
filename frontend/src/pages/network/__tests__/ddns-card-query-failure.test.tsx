import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
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

function mockDDNSFailure() {
  server.use(
    http.get(API_ROUTES.network.ddns, () =>
      HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
    ),
  );
}

describe('DdnsCard when the ddns config GET fails', () => {
  it('reports the failure instead of claiming ddns-scripts is missing', async () => {
    mockDDNSFailure();
    renderCard();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    // The install advice is a confident statement about the router; it must not
    // appear for a request that never succeeded.
    expect(screen.queryByText(/Dynamic DNS is unavailable/)).not.toBeInTheDocument();
    expect(screen.queryByText(/opkg install ddns-scripts/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Save DDNS Settings/ })).not.toBeInTheDocument();
  });
});
