import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import {
  mockServices,
} from '../data';

export const servicesHandlers = [
  http.get(API_ROUTES.services.list, () => {
    return HttpResponse.json(mockServices);
  })

  http.post(API_ROUTES.services.install, () => {
    return HttpResponse.json({ success: true });
  })

  http.post(`${API_ROUTES.services.installStream.replace(':id', ':id')}`, ({ params }) => {
    const id = params.id as string;
    const body = [
      JSON.stringify({ type: 'log', data: `Installing package: ${id}` }),
      JSON.stringify({ type: 'log', data: `Fetching ${id}...` }),
      JSON.stringify({ type: 'log', data: `Package ${id} installed successfully` }),
      JSON.stringify({ type: 'done' }),
    ].join('\n');
    return new HttpResponse(body, {
      headers: { 'Content-Type': 'application/x-ndjson' },
    });
  })

  http.post(API_ROUTES.services.remove, () => {
    return HttpResponse.json({ success: true });
  })

  http.post(`${API_ROUTES.services.removeStream.replace(':id', ':id')}`, ({ params }) => {
    const id = params.id as string;
    const body = [
      JSON.stringify({ type: 'log', data: `Removing package: ${id}` }),
      JSON.stringify({ type: 'log', data: `Package ${id} removed successfully` }),
      JSON.stringify({ type: 'done' }),
    ].join('\n');
    return new HttpResponse(body, {
      headers: { 'Content-Type': 'application/x-ndjson' },
    });
  })

  http.post(API_ROUTES.services.start, () => {
    return HttpResponse.json({ success: true });
  })

  http.post(API_ROUTES.services.stop, () => {
    return HttpResponse.json({ success: true });
  })

  http.post(API_ROUTES.services.autostart, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
