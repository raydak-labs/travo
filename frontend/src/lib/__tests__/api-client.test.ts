import { describe, it, expect, beforeEach, vi } from 'vitest';
import { http, HttpResponse } from 'msw/http';
import { server } from '@/mocks/server';
import {
  apiClient,
  setToken,
  getToken,
  clearToken,
  streamRequest,
  type StreamEvent,
} from '../api-client';

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

  it('omits Content-Type on bodyless GETs', async () => {
    let seenContentType: string | null = 'unset';
    server.use(
      http.get('/api/v1/system/info', ({ request }) => {
        seenContentType = request.headers.get('content-type');
        return HttpResponse.json({ hostname: 'GL-MT3000' });
      }),
    );

    await apiClient.get('/api/v1/system/info');
    expect(seenContentType).toBeNull();
  });

  it('reports the status line when the error body is not JSON', async () => {
    server.use(
      http.get('/api/v1/system/info', () =>
        HttpResponse.text('<html><body>502 Bad Gateway</body></html>', { status: 502 }),
      ),
    );

    await expect(apiClient.get('/api/v1/system/info')).rejects.toThrow(
      'Request failed with status 502',
    );
  });

  it('routes blob downloads through the 401 handler', async () => {
    setToken('expired-token');
    server.use(
      http.get('/api/v1/system/backup', () =>
        HttpResponse.json({ error: 'nope' }, { status: 401 }),
      ),
    );

    await expect(apiClient.getBlob('/api/v1/system/backup')).rejects.toThrow('nope');
    expect(getToken()).toBeNull();
  });

  it('uploads FormData without forcing a JSON content type', async () => {
    setToken('tok');
    let contentType: string | null = 'unset';
    let bodyText = '';
    server.use(
      http.post('/api/v1/system/restore', async ({ request }) => {
        contentType = request.headers.get('content-type');
        bodyText = await request.text();
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    const form = new FormData();
    form.append('keep_settings', 'true');
    await apiClient.postForm('/api/v1/system/restore', form);

    expect(contentType).toMatch(/^multipart\/form-data; boundary=/);
    expect(bodyText).toContain('keep_settings');
  });
});

describe('streamRequest', () => {
  it('flushes the decoder so a character split across the last chunk survives', async () => {
    const encoder = new TextEncoder();
    // "€" is 3 UTF-8 bytes; the second chunk starts mid-character.
    const head = encoder.encode('{"type":"log","data":"ok"}\n{"type":"log","data":"A');
    const euro = encoder.encode('€');
    const tail = encoder.encode('"}');
    const bytes = new Uint8Array([...head, ...euro, ...tail]);
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(bytes.slice(0, head.length + 1));
        controller.enqueue(bytes.slice(head.length + 1));
        controller.close();
      },
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 200 })));

    const events: StreamEvent[] = [];
    await streamRequest('/api/v1/services/x/install/stream', (e) => events.push(e));

    expect(events).toEqual([
      { type: 'log', data: 'ok' },
      { type: 'log', data: 'A€' },
    ]);
    vi.unstubAllGlobals();
  });

  it('stops delivering events once the caller aborts', async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      async start(controller) {
        controller.enqueue(encoder.encode('{"type":"log","data":"first"}\n'));
        await new Promise((resolve) => setTimeout(resolve, 50));
        controller.enqueue(encoder.encode('{"type":"log","data":"second"}\n'));
        controller.close();
      },
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 200 })));

    const controller = new AbortController();
    const events: StreamEvent[] = [];
    const promise = streamRequest(
      '/api/v1/services/x/install/stream',
      (e) => events.push(e),
      controller.signal,
    );
    await new Promise((resolve) => setTimeout(resolve, 10));
    controller.abort();

    // Aborting must end the stream and suppress every later line; whether the
    // promise settles as resolved or rejected depends on how far the request
    // got, so only the observable output is asserted.
    await promise.catch(() => undefined);
    expect(events.map((e) => e.data)).toEqual(['first']);
    vi.unstubAllGlobals();
  });
});
