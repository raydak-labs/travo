import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { router } from '@/router';
import { setToken, clearToken } from '@/lib/api-client';
import { resetSetupStatusCache } from '@/lib/setup-status';

/**
 * An unknown path must produce the styled 404 page.
 *
 * This regressed once already: the catch-all was written as a `path: '*'` child
 * of the pathless `protectedRoute`, which TanStack never reached, so an unknown
 * URL rendered the router's bare default "Not Found" text with no shell, no
 * card and no way onward. The bug was invisible to every other test because no
 * test navigated to an unknown path.
 */
describe('unknown routes', () => {
  it('renders the styled not-found page, not the router default', async () => {
    resetSetupStatusCache();
    setToken('test-token');
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider
          router={router}
          history={createMemoryHistory({ initialEntries: ['/definitely-not-a-page'] })}
        />
      </QueryClientProvider>,
    );

    expect(await screen.findByText('Page not found')).toBeInTheDocument();
    // The two ways onward, which the router default offered none of.
    expect(screen.getByRole('link', { name: /go to dashboard/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /check logs/i })).toBeInTheDocument();

    resetSetupStatusCache();
    clearToken();
  });
});
