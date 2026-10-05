import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/mocks/server';
import { LogsPage } from '../logs-page';

// "No log entries" reads as a quiet system. A traveller on a bad link must be
// able to tell that apart from a request that never reached the router.
describe('LogsPage when the log request fails', () => {
  it('reports the failure instead of an empty log', async () => {
    server.use(
      http.get(API_ROUTES.system.logs, () =>
        HttpResponse.json({ error: 'logd busy' }, { status: 500 }),
      ),
    );
    renderWithProviders(<LogsPage />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('logd busy');
    });
    expect(screen.queryByText('No log entries')).not.toBeInTheDocument();
  });

  it('reports the failure for the kernel tab too', async () => {
    server.use(
      http.get(API_ROUTES.system.kernelLogs, () =>
        HttpResponse.json({ error: 'dmesg busy' }, { status: 500 }),
      ),
    );
    const user = (await import('@testing-library/user-event')).default.setup();
    renderWithProviders(<LogsPage />);

    await waitFor(() => {
      expect(screen.getByTestId('log-content')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Kernel Log' }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('dmesg busy');
    });
    expect(screen.queryByText('No log entries')).not.toBeInTheDocument();
  });
});
