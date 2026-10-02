import { describe, expect, it, beforeEach } from 'vitest';
import { server } from '@/mocks/server';
import { MIN_PASSWORD_LENGTH } from '@shared/index';

/**
 * The change-password contract is split across three places: the backend
 * handler, shared/src/api/auth.ts, and the msw mock the whole frontend test
 * suite runs against. A mock that drifts from the real response does not fail
 * any test — it silently produces wrong behaviour in every test that touches
 * the flow, because the client stores whatever the response contains.
 */
describe('change-password mock contract', () => {
  beforeEach(() => {
    server.resetHandlers();
  });

  async function changePassword(current_password: string, new_password: string) {
    const { apiClient } = await import('@/lib/api-client');
    return apiClient.put<Record<string, unknown>>('/api/v1/auth/password', {
      current_password,
      new_password,
    });
  }

  it('returns a usable token, because the client stores it', async () => {
    const res = await changePassword('admin', 'a-long-enough-password');

    // The backend revokes every session including the caller's and issues a
    // replacement; the client does setToken(res.token). A response without a
    // token therefore stores the string "undefined" as the session token.
    expect(typeof res.token).toBe('string');
    expect((res.token as string).length).toBeGreaterThan(0);
    expect(res.status).toBe('ok');
    expect(typeof res.revoked_sessions).toBe('number');
  });

  it('enforces the same minimum length as the backend policy', async () => {
    const tooShort = 'a'.repeat(MIN_PASSWORD_LENGTH - 1);
    await expect(changePassword('admin', tooShort)).rejects.toThrow();

    const longEnough = 'a'.repeat(MIN_PASSWORD_LENGTH);
    await expect(changePassword('admin', longEnough)).resolves.toBeTruthy();
  });

  it('still rejects a wrong current password', async () => {
    await expect(changePassword('not-admin', 'a-long-enough-password')).rejects.toThrow();
  });
});
