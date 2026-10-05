import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';

function renderCard() {
  server.use(
    http.get(API_ROUTES.network.connectionMethod, () =>
      HttpResponse.json({ method: 'wired', interface: 'eth0' }),
    ),
  );
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <WifiModeCard />
    </QueryClientProvider>,
  );
}

describe('WifiModeCard', () => {
  it('keeps Recommended/Active badges inside mode tiles on narrow layouts', async () => {
    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Recommended')).toBeInTheDocument();
    });

    const recommended = screen.getByText('Recommended');
    expect(recommended.className).toMatch(/shrink-0/);

    const tile = recommended.closest('button');
    expect(tile).toBeTruthy();
    expect(tile!.className).toMatch(/overflow-visible/);
    expect(tile!.className).toMatch(/min-w-0/);

    const badgeRow = recommended.parentElement;
    expect(badgeRow?.className).toMatch(/flex-wrap/);
  });

  // finalizeWifiMutation polls POST /wifi/apply/confirm for up to 30s. Before,
  // the dialog closed on the click and the page showed three disabled tiles for
  // the whole wait.
  it('shows the confirm-window progress dialog while the mode switch is applied', async () => {
    const user = userEvent.setup();
    let releaseConfirm: (() => void) | undefined;
    server.use(
      http.post(API_ROUTES.wifi.applyConfirm, async () => {
        await new Promise<void>((resolve) => {
          releaseConfirm = resolve;
        });
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Recommended')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /Access Point/ }));
    await user.click(await screen.findByRole('button', { name: /I understand, switch mode/ }));

    const progress = await screen.findByRole('dialog');
    expect(progress.textContent).toContain('Switching to Access Point mode');
    expect(progress.textContent).toContain('This can take up to 30 seconds.');
    expect(progress.textContent).toContain('rolls back to the previous mode');
    expect(progress.textContent).toContain('Elapsed:');

    releaseConfirm?.();
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
  });
});
