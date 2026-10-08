import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES, type ConnectionMethod } from '@shared/index';
import { server } from '@/mocks/server';
import { WiFiScheduleCard } from '../wifi-schedule-card';

function renderCard(method: ConnectionMethod['method'] = 'ethernet') {
  server.use(
    http.get(API_ROUTES.network.connectionMethod, () =>
      HttpResponse.json({ method, interface: method === 'ethernet' ? 'eth0' : 'wlan0' }),
    ),
  );

  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnMount: false, refetchOnWindowFocus: false },
    },
  });

  render(
    <QueryClientProvider client={client}>
      <WiFiScheduleCard />
    </QueryClientProvider>,
  );
}

describe('WiFiScheduleCard', () => {
  it('saves an enabled schedule over ethernet without a warning', async () => {
    const user = userEvent.setup();
    const saved: unknown[] = [];
    server.use(
      http.put(API_ROUTES.wifi.schedule, async ({ request }) => {
        saved.push(await request.json());
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderCard('ethernet');

    await user.click(await screen.findByRole('checkbox', { name: /enable schedule/i }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(saved).toHaveLength(1);
    });
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  // Saving an enabled schedule at 21:00 with "Off at 22:00" disconnects a
  // WiFi-connected admin at 22:00 with no route back to this page.
  it('warns a wifi-connected operator that saving disconnects them, and saves on confirm', async () => {
    const user = userEvent.setup();
    const saved: unknown[] = [];
    server.use(
      http.put(API_ROUTES.wifi.schedule, async ({ request }) => {
        saved.push(await request.json());
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderCard('wifi-client');

    await user.click(await screen.findByRole('checkbox', { name: /enable schedule/i }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    const dialog = await screen.findByRole('dialog');
    expect(dialog.textContent).toContain("this router's WiFi turns off");
    expect(dialog.textContent).toMatch(/until \d{2}:\d{2}/);
    expect(dialog.textContent).toContain('Ethernet');
    expect(saved).toEqual([]);

    await user.click(screen.getByRole('button', { name: 'Save anyway' }));

    await waitFor(() => {
      expect(saved).toEqual([{ enabled: true, on_time: '07:00', off_time: '23:00' }]);
    });
  });

  it('warns an AP-connected operator too, because the toggle takes every radio down', async () => {
    const user = userEvent.setup();
    const saved: unknown[] = [];
    server.use(
      http.put(API_ROUTES.wifi.schedule, async ({ request }) => {
        saved.push(await request.json());
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderCard('wifi-ap');

    await user.click(await screen.findByRole('checkbox', { name: /enable schedule/i }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    const dialog = await screen.findByRole('dialog');
    expect(dialog.textContent).toContain("this router's WiFi turns off");
    expect(saved).toEqual([]);
  });

  it('saves without a warning when the operator cancels', async () => {
    const user = userEvent.setup();
    const saved: unknown[] = [];
    server.use(
      http.put(API_ROUTES.wifi.schedule, async ({ request }) => {
        saved.push(await request.json());
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderCard('wifi-client');

    await user.click(await screen.findByRole('checkbox', { name: /enable schedule/i }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await user.click(await screen.findByRole('button', { name: 'Cancel' }));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    expect(saved).toEqual([]);
  });
});
