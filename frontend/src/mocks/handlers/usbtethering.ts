import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';

export const usbtetheringHandlers = [
  http.get(API_ROUTES.network.usbTethering, () => {
    return HttpResponse.json({
      detected: false,
      device_type: '',
      interface: '',
      is_up: false,
      ip_address: '',
      configured: false,
    });
  }),

  http.post(API_ROUTES.network.usbTetheringConfigure, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.network.usbTetheringUnconfigure, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
