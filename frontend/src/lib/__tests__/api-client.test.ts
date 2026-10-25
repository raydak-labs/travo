import { describe, it, expect, beforeEach, vi } from 'vitest';
import { http, HttpResponse } from 'msw/http';
import { server } from '@/mocks/server';
import { login, restoreBackup } from '@/api/endpoints';
import {
  apiFetch,
  getBlob,
  setToken,
  getToken,
  clearToken,
  streamRequest,
  type StreamEvent,
} from '../api-client';

beforeEach(() => {
  clearToken();
});

describe('apiFetch', () => {
  it('formats GET request correctly', async () => {
    const data = await apiFetch<{ hostname: string }>('/api/v1/system/info', {});
    expect(data).toHaveProperty('hostname');
    expect(data.hostname).toBe('GL-MT3000');
  });

  it('includes auth token when set', async () => {
    setToken('test-token-123');
    expect(getToken()).toBe('test-token-123');

    // The request should succeed since MSW doesn't check auth
    const data = await apiFetch<{ hostname: string }>('/api/v1/system/info', {});
    expect(data).toHaveProperty('hostname');
  });

  it('throws on non-2xx response', async () => {
    await expect(login({ password: 'wrong' })).rejects.toThrow('Invalid password');
  });

  it('clears token on 401 response (non-login endpoint)', async () => {
    setToken('expired-token');
    expect(getToken()).toBe('expired-token');

    server.use(
      http.get('/api/v1/system/info', () => {
        return HttpResponse.json({ error: 'Unauthorized' }, { status: 401 });
      }),
    );

    await expect(apiFetch('/api/v1/system/info', {})).rejects.toThrow('Unauthorized');
    // handleUnauthorized clears the token and attempts redirect to /login
    expect(getToken()).toBeNull();
  });

  it('does not clear token on 401 for login endpoint', async () => {
    setToken('existing-token');

    await expect(login({ password: 'wrong' })).rejects.toThrow('Invalid password');
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

    await apiFetch('/api/v1/system/info', {});
    expect(seenContentType).toBeNull();
  });

  it('reports the status line when the error body is not JSON', async () => {
    server.use(
      http.get('/api/v1/system/info', () =>
        HttpResponse.text('<html><body>502 Bad Gateway</body></html>', { status: 502 }),
      ),
    );

    await expect(apiFetch('/api/v1/system/info', {})).rejects.toThrow(
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

    await expect(getBlob('/api/v1/system/backup')).rejects.toThrow('nope');
    expect(getToken()).toBeNull();
  });

  // The browser must set the multipart boundary itself, so neither the
  // generated function nor the client may add a Content-Type.
  it('uploads FormData without forcing a content type', async () => {
    setToken('tok');
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ status: 'ok' }));
    vi.stubGlobal('fetch', fetchMock);

    await restoreBackup({ backup: new File(['archive'], 'backup.tar.gz') });

    const init = fetchMock.mock.calls[0][1] as RequestInit;
    const headers = new Headers(init.headers);
    expect(init.body).toBeInstanceOf(FormData);
    expect((init.body as FormData).has('backup')).toBe(true);
    expect(headers.get('content-type')).toBeNull();
    expect(headers.get('authorization')).toBe('Bearer tok');
    vi.unstubAllGlobals();
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
