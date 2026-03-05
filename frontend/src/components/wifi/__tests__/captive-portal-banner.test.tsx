import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { CaptivePortalBanner } from '../captive-portal-banner';
import { http, HttpResponse } from 'msw';
import { server } from '@/mocks/server';
import { mockCaptivePortalDetected } from '@/mocks/data';

function renderBanner() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <CaptivePortalBanner />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('CaptivePortalBanner', () => {
  it('shows banner when portal detected', async () => {
    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json(mockCaptivePortalDetected);
      }),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Login required to access internet')).toBeInTheDocument();
    });
  });

  it('hides when portal not detected', async () => {
    renderBanner();

    // Default mock returns detected: false
    await waitFor(() => {
      expect(screen.queryByText('Login required to access internet')).not.toBeInTheDocument();
    });
  });

  it('opens portal URL on button click', async () => {
    const user = userEvent.setup();
    const windowOpen = vi.spyOn(window, 'open').mockImplementation(() => null);

    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json(mockCaptivePortalDetected);
      }),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Open Login Page')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Open Login Page'));
    expect(windowOpen).toHaveBeenCalledWith('http://captive.hotel.com/login', '_blank');

    windowOpen.mockRestore();
  });

  it('dismisses on clicking dismiss button', async () => {
    const user = userEvent.setup();

    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json(mockCaptivePortalDetected);
      }),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Login required to access internet')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Dismiss'));

    await waitFor(() => {
      expect(screen.queryByText('Login required to access internet')).not.toBeInTheDocument();
    });
  });
});
