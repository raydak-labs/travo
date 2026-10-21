import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import type { AdGuardConfig } from '@shared/index';

// Fixtures for routes that had no mock until now. They live here rather than in
// mocks/data.ts because that file belongs to another lane's change set.

const mockAdGuardConfig: AdGuardConfig = {
  content: 'dns:\n  bind_hosts:\n    - 0.0.0.0\n  port: 53\nfilters: []\n',
};

export const adguardHandlers = [
  http.get(API_ROUTES.adguard.dns, () => {
    return HttpResponse.json({ enabled: false, dns_port: 5353 });
  }),

  http.put(API_ROUTES.adguard.dns, async ({ request }) => {
    const body = (await request.json()) as { enabled: boolean };
    return HttpResponse.json({ status: 'ok', enabled: body.enabled });
  }),

  http.get(API_ROUTES.adguard.dnsMode, () => {
    return HttpResponse.json({
      mode: 'adguard-forwarding',
      description: 'dnsmasq forwards DNS to AdGuard Home (port 5353)',
      adguard_running: true,
      dns_bypassed: false,
    });
  }),

  http.get(API_ROUTES.adguard.config, () => {
    return HttpResponse.json(mockAdGuardConfig);
  }),

  http.put(API_ROUTES.adguard.config, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.adguard.password, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
