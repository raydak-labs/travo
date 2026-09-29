import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '@/mocks/server';
import { setToken, clearToken } from '@/lib/api-client';
import {
  getSetupComplete,
  resetSetupStatusCache,
  SetupStatusUnavailableError,
} from '@/lib/setup-status';
import { requireSetupComplete } from '../route-guards';

/** The redirect thrown by TanStack carries its destination in `options.to`. */
function redirectTarget(error: unknown): string | undefined {
  return (error as { options?: { to?: string } } | null)?.options?.to;
}

beforeEach(() => {
  resetSetupStatusCache();
  setToken('test-token');
});

afterEach(() => {
  resetSetupStatusCache();
  clearToken();
});

describe('requireSetupComplete', () => {
  it('redirects to /login without a session', async () => {
    clearToken();
    const err = await requireSetupComplete().catch((e: unknown) => e);
    expect(redirectTarget(err)).toBe('/login');
  });

  it('redirects to /setup when setup is incomplete', async () => {
    server.use(
      http.get('/api/v1/system/setup-complete', () => HttpResponse.json({ complete: false })),
    );

    const err = await requireSetupComplete().catch((e: unknown) => e);
    expect(redirectTarget(err)).toBe('/setup');
  });

  it('resolves when setup is complete', async () => {
    server.use(
      http.get('/api/v1/system/setup-complete', () => HttpResponse.json({ complete: true })),
    );

    await expect(requireSetupComplete()).resolves.toBeUndefined();
  });

  it('propagates a 500 instead of rendering the protected route', async () => {
    server.use(
      http.get('/api/v1/system/setup-complete', () =>
        HttpResponse.json({ error: 'database is locked' }, { status: 500 }),
      ),
    );

    await expect(requireSetupComplete()).rejects.toBeInstanceOf(SetupStatusUnavailableError);
  });

  it('caches the answer across navigations', async () => {
    let calls = 0;
    server.use(
      http.get('/api/v1/system/setup-complete', () => {
        calls += 1;
        return HttpResponse.json({ complete: true });
      }),
    );

    await requireSetupComplete();
    await requireSetupComplete();
    await requireSetupComplete();

    expect(calls).toBe(1);
  });

  it('does not cache a failed check', async () => {
    let calls = 0;
    server.use(
      http.get('/api/v1/system/setup-complete', () => {
        calls += 1;
        return HttpResponse.json({ error: 'boom' }, { status: 502 });
      }),
    );

    await expect(getSetupComplete()).rejects.toBeInstanceOf(SetupStatusUnavailableError);
    await expect(getSetupComplete()).rejects.toBeInstanceOf(SetupStatusUnavailableError);

    expect(calls).toBe(2);
  });
});
