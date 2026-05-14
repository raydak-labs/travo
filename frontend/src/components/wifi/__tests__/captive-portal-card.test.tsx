import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { CaptivePortalCard } from '../captive-portal-card';
import { http, HttpResponse } from 'msw';
import { server } from '@/mocks/server';
import { mockCaptivePortalDetected } from '@/mocks/data';

function renderCard() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <CaptivePortalCard />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('CaptivePortalCard', () => {
  it('shows Login Required when portal detected and sta_connected', async () => {
    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json(mockCaptivePortalDetected);
      }),
    );

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Login Required')).toBeInTheDocument();
    });
  });

  it('shows Connected when internet ok', async () => {
    renderCard();

    // Default mock returns detected: false, can_reach_internet: true, sta_connected: true
    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeInTheDocument();
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

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Open Login')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Open Login'));
    expect(windowOpen).toHaveBeenCalledWith('http://captive.hotel.com/login', '_blank');

    windowOpen.mockRestore();
  });

  it('shows No Internet when no portal and no internet but sta_connected', async () => {
    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json({
          detected: false,
          can_reach_internet: false,
          dns_bypassed: false,
          dns_bypass_needed: false,
          sta_connected: true,
        });
      }),
    );

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('No Internet')).toBeInTheDocument();
    });
  });

  it('shows No Upstream when sta_connected is false', async () => {
    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json({
          detected: false,
          can_reach_internet: false,
          dns_bypassed: false,
          dns_bypass_needed: false,
          sta_connected: false,
        });
      }),
    );

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('No Upstream')).toBeInTheDocument();
    });
  });

  it('does NOT show Login Required when sta_connected is false', async () => {
    server.use(
      http.get('/api/v1/captive/status', () => {
        return HttpResponse.json({
          detected: true,
          portal_url: 'http://captive.hotel.com/login',
          can_reach_internet: false,
          dns_bypassed: false,
          dns_bypass_needed: false,
          sta_connected: false,
        });
      }),
    );

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('No Upstream')).toBeInTheDocument();
    });

    expect(screen.queryByText('Login Required')).not.toBeInTheDocument();
  });
});
