import { http, HttpResponse } from 'msw/http';
import { API_ROUTES, MIN_PASSWORD_LENGTH } from '@shared/index';

export const authHandlers = [
  http.post(API_ROUTES.auth.login, async ({ request }) => {
    const body = (await request.json()) as { password: string };
    if (body.password === 'admin') {
      return HttpResponse.json({
        token: 'mock-jwt-token-abc123',
        expires_at: '2026-03-05T00:00:00Z',
        expires_in: 86400,
      });
    }
    return HttpResponse.json({ error: 'Invalid password' }, { status: 401 });
  }),

  http.post(API_ROUTES.auth.logout, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.auth.session, () => {
    return HttpResponse.json({ valid: true, expires_in: 86400 });
  }),

  http.put(API_ROUTES.auth.password, async ({ request }) => {
    const body = (await request.json()) as { current_password: string; new_password: string };
    if (body.current_password !== 'admin') {
      return HttpResponse.json({ error: 'invalid current password' }, { status: 401 });
    }
    // Must match the backend: a change revokes every session (the caller's
    // included) and returns a replacement token, which the client MUST store.
    // A response without `token` makes the client persist "undefined" as its
    // session token. See shared/src/api/auth.ts ChangePasswordResponse.
    if (body.new_password.length < MIN_PASSWORD_LENGTH) {
      return HttpResponse.json(
        { error: `new password must be at least ${MIN_PASSWORD_LENGTH} characters` },
        { status: 400 },
      );
    }
    return HttpResponse.json({
      status: 'ok',
      token: 'mock-rotation-token',
      expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
      expires_in: 24 * 60 * 60,
      revoked_sessions: 1,
    });
  }),
];
