import { session } from '../session.svelte';
import { csrfInterceptor } from './csrf';
import { type Client, createClient } from './gen/client';
import { client } from './gen/client.gen';

// A 401 from these is a wrong password, not an expired session.
const AUTH_ENDPOINTS = ['/api/v1/auth/login', '/api/v1/setup/password'];

export function isAuthEndpoint(url: string): boolean {
  const { pathname } = new URL(url, 'http://localhost');
  return AUTH_ENDPOINTS.includes(pathname);
}

/** Response interceptor: any other 401 means the session is gone, so the guard must show login. */
export function sessionExpiryInterceptor(response: Response, request: Request): Response {
  if (response.status === 401 && !isAuthEndpoint(request.url)) session.expire();
  return response;
}

client.setConfig({ baseUrl: '', credentials: 'same-origin' });
client.interceptors.request.use(csrfInterceptor);
client.interceptors.response.use(sessionExpiryInterceptor);

export { client };

/** A client with the same transport but none of the global response interceptors. */
export function scopedAPIClient(): Client {
  return createClient({ ...client.getConfig(), credentials: 'same-origin' });
}
