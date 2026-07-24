import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';

function renderCard() {
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
});
