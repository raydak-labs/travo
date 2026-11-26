import { describe, expect, it, vi } from 'vitest';
import { session } from '../session.svelte';
import { client, isAuthEndpoint, scopedAPIClient } from './client';

describe('isAuthEndpoint', () => {
  it.each([
    ['http://x.test/api/v1/auth/login', true],
    ['http://x.test/api/v1/setup/password', true],
    ['/api/v1/auth/login', true],
    ['http://x.test/api/v1/auth/session', false],
    ['http://x.test/api/v1/auth/password', false],
    ['http://x.test/api/v1/auth/logout', false],
    ['http://x.test/api/v1/auth/login/extra', false],
  ])('%s -> %s', (url, want) => {
    expect(isAuthEndpoint(url)).toBe(want);
  });
});

describe('scopedAPIClient', () => {
  it('inherits transport and same-origin credentials without the global expiry interceptor', async () => {
    const fetchFn = vi.fn(async (_request: Request) => jsonResponse());
    client.setConfig({ baseUrl: 'http://daemon.test', fetch: fetchFn as unknown as typeof fetch });
    const expire = vi.spyOn(session, 'expire').mockImplementation(() => {});
    const scoped = scopedAPIClient();
    await scoped.get({ url: '/api/v1/dashboard/connectivity' });
    expect(expire).not.toHaveBeenCalled();
    await client.get({ url: '/api/v1/dashboard/connectivity' });
    expect(expire).toHaveBeenCalledTimes(1);
    const request = fetchFn.mock.calls[0]?.[0];
    expect(request?.credentials).toBe('same-origin');
    expire.mockRestore();
  });
});

function jsonResponse(): Response {
  return new Response(JSON.stringify({ error: 'gone', code: 'unauthenticated' }), {
    status: 401,
    headers: { 'Content-Type': 'application/json' },
  });
}
