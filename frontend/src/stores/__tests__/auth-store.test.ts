import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { http, HttpResponse } from 'msw/http';
import { server } from '@/mocks/server';
import { useAuthStore } from '@/stores/auth-store';
import { getToken, setToken } from '@/lib/api-client';

describe('auth store', () => {
  beforeEach(() => {
    setToken('session-token');
    useAuthStore.setState({ token: 'session-token', isAuthenticated: true });
  });

  afterEach(() => {
    server.resetHandlers();
  });

  it('revokes the session server-side on logout', async () => {
    const seen: { auth: string | null; calls: number } = { auth: null, calls: 0 };
    server.use(
      http.post('/api/v1/auth/logout', ({ request }) => {
        seen.calls += 1;
        seen.auth = request.headers.get('authorization');
        return HttpResponse.json({ success: true });
      }),
    );

    useAuthStore.getState().logout();
    await vi.waitFor(() => expect(seen.calls).toBe(1));

    // The revoke request must still carry the token being revoked.
    expect(seen.auth).toBe('Bearer session-token');
    expect(getToken()).toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });

  it('clears local state even when revocation fails', async () => {
    server.use(
      http.post('/api/v1/auth/logout', () => HttpResponse.json({ error: 'boom' }, { status: 500 })),
    );

    useAuthStore.getState().logout();
    await vi.waitFor(() => expect(getToken()).toBeNull());

    expect(useAuthStore.getState().token).toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });
});
