import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { mockSQMConfig } from '../data';

export const sqmHandlers = [
  http.get(API_ROUTES.sqm.config, () => {
    return HttpResponse.json(mockSQMConfig);
  }),

  http.put(API_ROUTES.sqm.config, async ({ request }) => {
    const body = (await request.json()) as typeof mockSQMConfig;
    return HttpResponse.json({ status: 'ok', saved: body });
  }),

  http.post(API_ROUTES.sqm.apply, () => {
    return HttpResponse.json({ ok: true, output: 'restarted' });
  }),
];
