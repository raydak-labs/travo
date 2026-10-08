import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES, type APConfig } from '@shared/index';
import { server } from '@/mocks/server';
import { mockAPConfigs } from '@/mocks/data';
import { APUnifiedConfigForm } from '../ap-unified-config-form';

interface PutCall {
  section: string;
  body: { ssid: string; encryption: string; key: string; enabled?: boolean };
}

function renderForm(aps: APConfig[] = mockAPConfigs) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnMount: false, refetchOnWindowFocus: false },
    },
  });
  const onEnabledChange = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <APUnifiedConfigForm
        apConfigs={aps}
        enabledBySection={{}}
        activeEnabledCount={aps.filter((a) => a.enabled).length}
        onEnabledChange={onEnabledChange}
      />
    </QueryClientProvider>,
  );
  return client;
}

describe('APUnifiedConfigForm', () => {
  it('writes every band with the same credentials', async () => {
    const user = userEvent.setup();
    const puts: PutCall[] = [];
    server.use(
      http.put(`${API_ROUTES.wifi.ap}/:section`, async ({ params, request }) => {
        puts.push({
          section: params.section as string,
          body: (await request.json()) as PutCall['body'],
        });
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderForm();
    await user.clear(screen.getByPlaceholderText('SSID for all radios'));
    await user.type(screen.getByPlaceholderText('SSID for all radios'), 'Holiday');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(puts.map((p) => p.section)).toEqual(['default_radio0', 'default_radio1']);
    });
    expect(puts.every((p) => p.body.ssid === 'Holiday')).toBe(true);
  });

  // The bands are separate PUTs that the device commits one by one. A failure on
  // the second used to leave the first band renamed with nothing to undo it.
  it('rolls the first band back when the second one fails, and names the failure', async () => {
    const user = userEvent.setup();
    const puts: PutCall[] = [];
    server.use(
      http.put(`${API_ROUTES.wifi.ap}/:section`, async ({ params, request }) => {
        const section = params.section as string;
        const body = (await request.json()) as PutCall['body'];
        puts.push({ section, body });
        if (section === 'default_radio1' && body.ssid === 'Holiday') {
          return HttpResponse.json({ error: 'uci commit failed' }, { status: 400 });
        }
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderForm();
    await user.clear(screen.getByPlaceholderText('SSID for all radios'));
    await user.type(screen.getByPlaceholderText('SSID for all radios'), 'Holiday');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(puts.length).toBe(3);
    });

    // 1: new credentials on band 1, 2: failure on band 2, 3: band 1 restored.
    expect(puts.map((p) => `${p.section}:${p.body.ssid}`)).toEqual([
      'default_radio0:Holiday',
      'default_radio1:Holiday',
      'default_radio0:OpenWrt-Travel',
    ]);
    expect(puts[2]!.body.key).toBe('travel12345');

    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain('Failed to save 5 GHz radio1 (default_radio1)');
    expect(alert.textContent).toContain('Rolled back to the previous settings on default_radio0');
  });
});
