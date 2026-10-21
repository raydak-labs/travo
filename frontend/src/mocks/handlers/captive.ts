import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { mockCaptivePortalStatus } from '../data';

export const captiveHandlers = [
  http.get(API_ROUTES.captive.status, () => {
    return HttpResponse.json(mockCaptivePortalStatus);
  }),

  http.post(API_ROUTES.captive.autoAccept, () => {
    return HttpResponse.json({
      ok: true,
      message: 'mock auto-accept',
      detected: false,
      can_reach_internet: true,
    });
  }),

  http.post(API_ROUTES.captive.dnsBypass, () => {
    return HttpResponse.json({ ok: true, message: 'DNS switched to upstream' });
  }),

  http.post(API_ROUTES.captive.dnsRestore, () => {
    return HttpResponse.json({ ok: true, message: 'DNS restored' });
  }),
];
