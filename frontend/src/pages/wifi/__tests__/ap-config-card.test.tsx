import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { mockAPConfigs } from '@/mocks/data';
import { APConfigCard } from '../ap-config-card';

function renderCard() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnMount: false, refetchOnWindowFocus: false } },
  });
  client.setQueryData(['wifi', 'ap'], mockAPConfigs);
  return render(
    <QueryClientProvider client={client}>
      <APConfigCard />
    </QueryClientProvider>,
  );
}

describe('APConfigCard', () => {
  it('defaults to one shared SSID field for multiple radios', async () => {
    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Access Point Configuration')).toBeInTheDocument();
    });

    expect(screen.getByPlaceholderText('SSID for all radios')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: /Different settings per radio/i })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: /^SSID$/i })).not.toBeInTheDocument();
  });

  it('shows per-radio forms when separate settings is enabled', async () => {
    const user = userEvent.setup();
    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Access Point Configuration')).toBeInTheDocument();
    });

    const toggle = screen.getByRole('checkbox', { name: /Different settings per radio/i });
    await user.click(toggle);

    await waitFor(() => {
      expect(screen.getAllByRole('textbox', { name: /^SSID$/i })).toHaveLength(2);
    });
  });
});
