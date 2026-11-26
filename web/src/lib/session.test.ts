import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { deferred } from './deferred.test-utils';

// Fresh module graph per test: the session is module state and must start `unknown`.
let session: typeof import('./session.svelte').session;
let csrf: typeof import('./api/csrf').csrf;
let client: typeof import('./api/gen/client.gen').client;
let getHealth: typeof import('./api/gen').getHealth;

// The generated client takes its transport from config: fake it, no network.
const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

type Route = Record<string, Response | Error>;
function serve(routes: Route) {
  const fetchFn = vi.fn(async (request: Request) => {
    const { pathname } = new URL(request.url);
    const hit = routes[pathname];
    if (!hit) throw new Error(`unexpected ${pathname}`);
    if (hit instanceof Error) throw hit;
    return hit.clone();
  });
  client.setConfig({ baseUrl: 'http://daemon.test', fetch: fetchFn as unknown as typeof fetch });
  return fetchFn;
}

const SETUP = '/api/v1/setup';
const SESSION = '/api/v1/auth/session';
const body = { expires_in: 100, csrf_token: 'tok' };

beforeEach(async () => {
  vi.resetModules();
  ({ session } = await import('./session.svelte'));
  ({ csrf } = await import('./api/csrf'));
  ({ client } = await import('./api/gen/client.gen'));
  ({ getHealth } = await import('./api/gen'));
  await import('./api/client'); // registers the CSRF and 401 interceptors on the client
});

afterEach(() => {
  session.expire(); // clears the pending timer
  vi.useRealTimers();
});

describe('session.bootstrap', () => {
  it('setup unset -> setup_required', async () => {
    serve({ [SETUP]: json(200, { password_set: false }) });
    await session.bootstrap();
    expect(session.state).toBe('setup_required');
  });

  it('session 200 -> authenticated with the CSRF token', async () => {
    serve({ [SETUP]: json(200, { password_set: true }), [SESSION]: json(200, body) });
    await session.bootstrap();
    expect(session.state).toBe('authenticated');
    expect(csrf.get()).toBe('tok');
  });

  it('session 401 -> anonymous, whatever the code', async () => {
    serve({
      [SETUP]: json(200, { password_set: true }),
      [SESSION]: json(401, { error: 'no', code: 'whatever' }),
    });
    await session.bootstrap();
    expect(session.state).toBe('anonymous');
    expect(session.error).toBeNull();
  });

  it.each([
    ['setup 503', { [SETUP]: json(503, { error: 'down', code: 'device_unavailable' }) }],
    ['setup network error', { [SETUP]: new Error('offline') }],
    [
      'session 503',
      {
        [SETUP]: json(200, { password_set: true }),
        [SESSION]: json(503, { error: 'down', code: 'device_unavailable' }),
      },
    ],
    [
      'session network error',
      { [SETUP]: json(200, { password_set: true }), [SESSION]: new Error('offline') },
    ],
  ])('%s -> stays unknown with an error', async (_name, routes) => {
    serve(routes);
    await session.bootstrap();
    expect(session.state).toBe('unknown');
    expect(session.error).toBeTruthy();
  });
});

describe('session expiry', () => {
  it('start and expire advance the reactive session generation', async () => {
    expect(session.generation).toBe(0);
    serve({ '/api/v1/auth/login': json(200, body) });
    await session.login('pw');
    expect(session.generation).toBe(1);
    session.expire();
    expect(session.generation).toBe(2);
    await session.login('pw');
    expect(session.generation).toBe(3);
  });

  it('expires after expires_in seconds: anonymous and CSRF cleared', async () => {
    vi.useFakeTimers();
    serve({ '/api/v1/auth/login': json(200, body) });
    expect(await session.login('pw')).toEqual({ ok: true });
    expect(session.state).toBe('authenticated');
    expect(csrf.get()).toBe('tok');

    vi.advanceTimersByTime(99_999);
    expect(session.state).toBe('authenticated');
    vi.advanceTimersByTime(1);
    expect(session.state).toBe('anonymous');
    expect(csrf.get()).toBeNull();
  });

  it('a new session reschedules the timer from the new expires_in', async () => {
    vi.useFakeTimers();
    serve({
      '/api/v1/auth/login': json(200, body),
      '/api/v1/auth/password': json(200, { expires_in: 500, csrf_token: 'tok2' }),
    });
    await session.login('pw');
    vi.advanceTimersByTime(50_000);
    expect(await session.changePassword('pw', 'new-password')).toEqual({ ok: true });
    expect(csrf.get()).toBe('tok2');
    vi.advanceTimersByTime(100_000);
    expect(session.state).toBe('authenticated');
    vi.advanceTimersByTime(400_000);
    expect(session.state).toBe('anonymous');
  });
});

describe('401 interceptor', () => {
  it('expires the session on a 401 from a normal route', async () => {
    serve({ '/api/v1/auth/login': json(200, body) });
    await session.login('pw');
    serve({ '/api/health': json(401, { error: 'gone', code: 'unauthenticated' }) });
    await getHealth();
    expect(session.state).toBe('anonymous');
    expect(csrf.get()).toBeNull();
  });

  it('does not expire on the 401 of /api/v1/auth/login (wrong password)', async () => {
    serve({ '/api/v1/auth/login': json(200, body) });
    await session.login('pw');
    serve({ '/api/v1/auth/login': json(401, { error: 'x', code: 'invalid_credentials' }) });
    expect(await session.login('bad')).toEqual({ ok: false, code: 'invalid_credentials' });
    expect(session.state).toBe('authenticated');
    expect(csrf.get()).toBe('tok');
  });
});

describe('session mutations', () => {
  it('reports the daemon code, and `network` when no response arrived', async () => {
    serve({ '/api/v1/setup/password': json(400, { error: 'x', code: 'invalid_password' }) });
    expect(await session.setup('password-1')).toEqual({ ok: false, code: 'invalid_password' });
    serve({ '/api/v1/setup/password': new Error('offline') });
    expect(await session.setup('password-1')).toEqual({ ok: false, code: 'network' });
  });

  it('logout ends the session even when the daemon refuses', async () => {
    serve({ '/api/v1/auth/login': json(200, body) });
    await session.login('pw');
    serve({ '/api/v1/auth/logout': new Error('offline') });
    await session.logout();
    expect(session.state).toBe('anonymous');
    expect(csrf.get()).toBeNull();
  });

  it('logout expires immediately and sends the pre-expiry CSRF token', async () => {
    serve({ '/api/v1/auth/login': json(200, body) });
    await session.login('pw');
    const generation = session.generation;
    const pending = deferred<Response>();
    const fetchFn = vi.fn(async (request: Request) => {
      expect(new URL(request.url).pathname).toBe('/api/v1/auth/logout');
      expect(request.headers.get('X-CSRF-Token')).toBe('tok');
      return pending.promise;
    });
    client.setConfig({ baseUrl: 'http://daemon.test', fetch: fetchFn as unknown as typeof fetch });
    const loggingOut = session.logout();
    expect(session.state).toBe('anonymous');
    expect(session.generation).toBe(generation + 1);
    expect(csrf.get()).toBeNull();
    pending.resolve(json(200, { status: 'ok' }));
    await loggingOut;
    expect(fetchFn).toHaveBeenCalledOnce();
  });

  // A stale setup state must not trap the user: the guard would bounce login <-> setup.
  it('already_set from setup re-learns the state: setup_required -> anonymous', async () => {
    serve({ [SETUP]: json(200, { password_set: false }) });
    await session.bootstrap();
    expect(session.state).toBe('setup_required');
    serve({
      '/api/v1/setup/password': json(409, { error: 'x', code: 'already_set' }),
      [SETUP]: json(200, { password_set: true }),
      [SESSION]: json(401, { error: 'x', code: 'unauthenticated' }),
    });
    expect(await session.setup('password-1')).toEqual({ ok: false, code: 'already_set' });
    expect(session.state).toBe('anonymous');
  });

  it('setup_required from login re-learns the state: anonymous -> setup_required', async () => {
    serve({ [SETUP]: json(200, { password_set: true }), [SESSION]: json(401, { error: 'x' }) });
    await session.bootstrap();
    expect(session.state).toBe('anonymous');
    serve({
      '/api/v1/auth/login': json(409, { error: 'x', code: 'setup_required' }),
      [SETUP]: json(200, { password_set: false }),
    });
    expect(await session.login('pw')).toEqual({ ok: false, code: 'setup_required' });
    expect(session.state).toBe('setup_required');
  });
});
