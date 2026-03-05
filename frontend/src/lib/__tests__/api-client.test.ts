import { describe, it, expect, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '@/mocks/server';
import { apiClient, setToken, getToken, clearToken } from '../api-client';

beforeEach(() => {
  clearToken();
});

describe('apiClient', () => {
  it('formats GET request correctly', async () => {
    const data = await apiClient.get<{ hostname: string }>('/api/v1/system/info');
    expect(data).toHaveProperty('hostname');
    expect(data.hostname).toBe('GL-MT3000');
  });

  it('includes auth token when set', async () => {
    setToken('test-token-123');
    expect(getToken()).toBe('test-token-123');

    // The request should succeed since MSW doesn't check auth
    const data = await apiClient.get<{ hostname: string }>('/api/v1/system/info');
    expect(data).toHaveProperty('hostname');
  });

  it('throws on non-2xx response', async () => {
    await expect(apiClient.post('/api/v1/auth/login', { password: 'wrong' })).rejects.toThrow(
      'Invalid password',
    );
  });

  it('clears token on 401 response (non-login endpoint)', async () => {
    setToken('expired-token');
    expect(getToken()).toBe('expired-token');

    server.use(
      http.get('/api/v1/system/info', () => {
        return HttpResponse.json({ error: 'Unauthorized' }, { status: 401 });
      }),
    );

    await expect(apiClient.get('/api/v1/system/info')).rejects.toThrow('Unauthorized');
    // handleUnauthorized clears the token and attempts redirect to /login
    expect(getToken()).toBeNull();
  });

  it('does not clear token on 401 for login endpoint', async () => {
    setToken('existing-token');

    await expect(apiClient.post('/api/v1/auth/login', { password: 'wrong' })).rejects.toThrow(
      'Invalid password',
    );
    // Login 401s should not trigger handleUnauthorized
    expect(getToken()).toBe('existing-token');
  });
});
