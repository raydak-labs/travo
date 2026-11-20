import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createClient } from './api/gen/client';
import { createDashboardEvents } from './dashboard-events.svelte';
import { deferred } from './deferred.test-utils';

const CONNECTIVITY = '/api/v1/dashboard/connectivity';
const RESOURCES = '/api/v1/system/info';
const THROUGHPUT = '/api/v1/dashboard/throughput';
const UPLINKS = '/api/v1/dashboard/uplinks';
const EVENTS = '/api/v1/dashboard/events';
const GETS = [CONNECTIVITY, RESOURCES, THROUGHPUT];

const connectivityBody = {
  active_interface: 'wan',
  medium: 'ethernet',
  carrier: true,
  internet: 'online',
};
const resourcesBody = { hostname: 'router', uptime_seconds: 10 };
// wwan as the daemon reports it from the AXT1800 interface dump (internal/device/mock/testdata).
const uplinksBody = {
  uplinks: [
    { name: 'wan', configured: true, up: false, device: null, ipv4: null, ssid: null, kind: null },
    {
      name: 'wwan',
      configured: true,
      up: true,
      device: 'phy0-sta0',
      ipv4: '192.168.90.241/24',
      ssid: null,
      kind: null,
    },
    {
      name: 'usbtether',
      configured: false,
      up: false,
      device: null,
      ipv4: null,
      ssid: null,
      kind: null,
    },
  ],
  failover: null,
};
const throughputBody = {
  interfaces: [{ name: 'wan', rx_bytes_per_second: 1, tx_bytes_per_second: 2 }],
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function bodyFor(path: string): Response {
  if (path === CONNECTIVITY) return json(connectivityBody);
  if (path === RESOURCES) return json(resourcesBody);
  if (path === THROUGHPUT) return json(throughputBody);
  if (path === UPLINKS) return json(uplinksBody);
  return json({ error: 'not found' }, 404);
}

function controlledSSE(contentType = 'text/event-stream') {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({
    start(stream) {
      controller = stream;
    },
  });
  const encoder = new TextEncoder();
  return {
    response: new Response(body, { status: 200, headers: { 'Content-Type': contentType } }),
    comment(text = 'connected') {
      controller.enqueue(encoder.encode(`: ${text}\n\n`));
    },
    event(name: string, data: unknown) {
      controller.enqueue(encoder.encode(`event: ${name}\ndata: ${JSON.stringify(data)}\n\n`));
    },
    close() {
      controller.close();
    },
    error(error: Error) {
      controller.error(error);
    },
  };
}

type Handler = (request: Request) => Response | Promise<Response>;

function harness(handler: Handler) {
  const requests: Request[] = [];
  const fetchFn = vi.fn(async (request: Request) => {
    requests.push(request);
    return handler(request);
  });
  const client = createClient({
    baseUrl: 'http://daemon.test',
    credentials: 'same-origin',
    fetch: fetchFn as unknown as typeof fetch,
  });
  let generation = 1;
  const expire = vi.fn(() => {
    generation += 1;
  });
  const dashboard = createDashboardEvents({
    client: () => client,
    generation: () => generation,
    expire,
  });
  return {
    dashboard,
    expire,
    requests,
    get generation() {
      return generation;
    },
    setGeneration(value: number) {
      generation = value;
    },
  };
}

type Harness = ReturnType<typeof harness>;
let active: Harness | undefined;

function paths(h: Harness, path: string): Request[] {
  return h.requests.filter((request) => new URL(request.url).pathname === path);
}

function current(): Harness {
  if (!active) throw new Error('test harness not initialized');
  return active;
}

async function flush(): Promise<void> {
  for (let i = 0; i < 12; i++) await Promise.resolve();
}

async function eventually(assertion: () => void): Promise<void> {
  for (let i = 0; i < 20; i++) {
    await flush();
    try {
      assertion();
      return;
    } catch (error) {
      if (i === 19) throw error;
    }
  }
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  active?.dashboard.stop();
  active = undefined;
  vi.useRealTimers();
});

describe('dashboard ownership and seeds', () => {
  it('opens no stream before authenticated shell ownership starts', async () => {
    active = harness(() => bodyFor(EVENTS));
    await vi.advanceTimersByTimeAsync(20_000);
    expect(active.requests).toHaveLength(0);
    expect(active.dashboard.streaming).toBe(false);
  });

  it('shares one subscription and seeds all three GETs with the ownership signal', async () => {
    const stream = controlledSSE();
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) return stream.response;
      return bodyFor(path);
    });
    active.dashboard.start(1);
    active.dashboard.start(1); // navigation must not replace shell ownership
    active.dashboard.start(1);
    await eventually(() => expect(active?.dashboard.streaming).toBe(true));

    expect(paths(active, EVENTS)).toHaveLength(1);
    for (const path of GETS) {
      expect(paths(active, path)).toHaveLength(1);
      expect(paths(active, path)[0]?.credentials).toBe('same-origin');
      expect(paths(active, path)[0]?.signal.aborted).toBe(false);
    }
    expect(active.dashboard.connectivity.data).toEqual(connectivityBody);
    expect(active.dashboard.resources.data).toEqual(resourcesBody);
    expect(active.dashboard.throughput.data).toEqual(throughputBody);

    active.dashboard.stop(1);
    for (const request of active.requests) expect(request.signal.aborted).toBe(true);
    active.dashboard.clear();
    expect(active.dashboard.connectivity.data).toBeUndefined();
    expect(active.dashboard.resources.data).toBeUndefined();
    expect(active.dashboard.throughput.data).toBeUndefined();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(paths(active, EVENTS)).toHaveLength(1);
  });

  it('password-change generation replacement aborts the old stream and pending retries', async () => {
    const first = controlledSSE();
    const second = controlledSSE();
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS)
        return paths(current(), EVENTS).length === 1 ? first.response : second.response;
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.dashboard.streaming).toBe(true));
    first.close();
    await eventually(() => expect(active?.dashboard.streaming).toBe(false));
    const oldRequests = [...active.requests];
    active.setGeneration(2);
    active.dashboard.start(2);
    await eventually(() => expect(paths(current(), EVENTS)).toHaveLength(2));
    for (const request of oldRequests) expect(request.signal.aborted).toBe(true);
    expect(active.dashboard.streaming).toBe(true);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(paths(active, EVENTS)).toHaveLength(2);
  });

  it('marks retained values stale while a page-cache restart waits for fresh data', async () => {
    const first = controlledSSE();
    const restored = controlledSSE();
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS)
        return paths(current(), EVENTS).length === 1 ? first.response : restored.response;
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.dashboard.throughput.status.state).toBe('ready'));

    active.dashboard.stop();
    active.dashboard.start(1);

    expect(active.dashboard.connectivity.data).toEqual(connectivityBody);
    expect(active.dashboard.resources.data).toEqual(resourcesBody);
    expect(active.dashboard.throughput.data).toEqual(throughputBody);
    expect(active.dashboard.connectivity.status).toEqual({ state: 'loading' });
    expect(active.dashboard.resources.status).toEqual({ state: 'loading' });
    expect(active.dashboard.throughput.status).toEqual({ state: 'loading' });
    await eventually(() => expect(active?.dashboard.throughput.status.state).toBe('ready'));
    expect(active.dashboard.connectivity.status.state).toBe('ready');
    expect(active.dashboard.resources.status.state).toBe('ready');
    expect(paths(active, EVENTS)).toHaveLength(2);
  });

  it('a current stream 401 expires once and never reconnects', async () => {
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      return path === EVENTS
        ? json({ error: 'gone', code: 'unauthenticated' }, 401)
        : bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.expire).toHaveBeenCalledTimes(1));
    expect(active.generation).toBe(2);
    await vi.advanceTimersByTimeAsync(20_000);
    expect(paths(active, EVENTS)).toHaveLength(1);
    for (const path of GETS) expect(paths(active, path)).toHaveLength(1);
  });

  it('ignores stale stream and seed 401s from a superseded generation', async () => {
    const oldEvents = deferred<Response>();
    const oldConnectivity = deferred<Response>();
    const replacement = controlledSSE();
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) {
        return paths(current(), EVENTS).length === 1 ? oldEvents.promise : replacement.response;
      }
      if (path === CONNECTIVITY && paths(current(), CONNECTIVITY).length === 1) {
        return oldConnectivity.promise;
      }
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await flush();
    active.setGeneration(2);
    active.dashboard.start(2);
    await eventually(() => expect(active?.dashboard.streaming).toBe(true));
    oldEvents.resolve(json({ error: 'old stream gone' }, 401));
    oldConnectivity.resolve(json({ error: 'old GET gone' }, 401));
    await flush();
    expect(active.expire).not.toHaveBeenCalled();
    expect(active.generation).toBe(2);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(paths(active, EVENTS)).toHaveLength(2);
  });
});

describe('dashboard event state', () => {
  it('keeps last good data stale on source_unavailable until recovery', async () => {
    const stream = controlledSSE();
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      return path === EVENTS ? stream.response : bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.dashboard.connectivity.data).toEqual(connectivityBody));
    stream.event('source_unavailable', { source: 'connectivity' });
    await eventually(() => expect(active?.dashboard.connectivity.status.state).toBe('error'));
    expect(active.dashboard.connectivity.data).toEqual(connectivityBody);
    const recovered = { ...connectivityBody, active_interface: 'phy0-sta0' };
    stream.event('connectivity', recovered);
    await eventually(() => expect(active?.dashboard.connectivity.data).toEqual(recovered));
    expect(active.dashboard.connectivity.status).toEqual({ state: 'ready' });
  });

  it('holds pushed uplinks and falls back to their GET while the stream is down', async () => {
    const stream = controlledSSE();
    let eventCalls = 0;
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) {
        eventCalls += 1;
        return eventCalls === 1 ? stream.response : json({ error: 'offline' }, 503);
      }
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.dashboard.streaming).toBe(true));
    const pushed = { ...uplinksBody, failover: { enabled: true, active: true, order: ['wan'] } };
    stream.event('uplinks', pushed);
    await eventually(() => expect(active?.dashboard.uplinks.data).toEqual(pushed));
    const seeded = paths(active, UPLINKS).length;
    stream.close();
    await eventually(() => expect(active?.dashboard.uplinks.status.state).toBe('error'));
    expect(active.dashboard.uplinks.data).toEqual(pushed);
    await vi.advanceTimersByTimeAsync(5_000);
    await eventually(() => expect(active?.dashboard.uplinks.data).toEqual(uplinksBody));
    expect(active.dashboard.uplinks.status).toEqual({ state: 'ready' });
    expect(paths(active, UPLINKS).length).toBeGreaterThan(seeded);
  });

  it.each(['success', 'failure'])(
    'ignores a late GET %s after a newer source event',
    async (outcome) => {
      const pending = deferred<Response>();
      const stream = controlledSSE();
      active = harness((request) => {
        const path = new URL(request.url).pathname;
        if (path === EVENTS) return stream.response;
        if (path === CONNECTIVITY) return pending.promise;
        return bodyFor(path);
      });
      active.dashboard.start(1);
      await eventually(() => expect(active?.dashboard.streaming).toBe(true));
      const pushed = { ...connectivityBody, active_interface: 'wwan0' };
      stream.event('connectivity', pushed);
      await eventually(() => expect(active?.dashboard.connectivity.data).toEqual(pushed));
      if (outcome === 'success')
        pending.resolve(json({ ...connectivityBody, active_interface: 'old' }));
      else pending.reject(new Error('late failure'));
      await flush();
      expect(active.dashboard.connectivity.data).toEqual(pushed);
      expect(active.dashboard.connectivity.status).toEqual({ state: 'ready' });
    },
  );

  it('marks every last value stale on EOF and refreshes each source through fallback', async () => {
    const stream = controlledSSE();
    let eventCalls = 0;
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) {
        eventCalls += 1;
        return eventCalls === 1 ? stream.response : json({ error: 'offline' }, 503);
      }
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.dashboard.throughput.data).toEqual(throughputBody));
    stream.close();
    await eventually(() => expect(active?.dashboard.throughput.status.state).toBe('error'));
    expect(active.dashboard.connectivity.status.state).toBe('error');
    expect(active.dashboard.resources.status.state).toBe('error');
    expect(active.dashboard.throughput.data).toEqual(throughputBody);
    await vi.advanceTimersByTimeAsync(5_000);
    await eventually(() => expect(active?.dashboard.throughput.status.state).toBe('ready'));
    expect(active.dashboard.connectivity.status).toEqual({ state: 'ready' });
    expect(active.dashboard.resources.status).toEqual({ state: 'ready' });
    for (const path of GETS) expect(paths(active, path).length).toBeGreaterThanOrEqual(2);
  });
});

describe('dashboard reconnect and disconnected fallback', () => {
  it.each([
    ['non-401 HTTP error', () => json({ error: 'full' }, 503)],
    ['invalid response type', () => json(connectivityBody)],
    [
      'network rejection',
      () => {
        throw new Error('offline');
      },
    ],
    [
      'unexpected EOF',
      () => {
        const stream = controlledSSE();
        stream.close();
        return stream.response;
      },
    ],
    [
      'read error',
      () => {
        const stream = controlledSSE();
        stream.error(new Error('reset'));
        return stream.response;
      },
    ],
  ])('retries after two seconds on %s', async (_name, failure) => {
    const healthy = controlledSSE();
    let calls = 0;
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) {
        calls += 1;
        return calls === 1 ? failure() : healthy.response;
      }
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await flush();
    await vi.advanceTimersByTimeAsync(1_999);
    expect(paths(active, EVENTS)).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    await eventually(() => expect(active?.dashboard.streaming).toBe(true));
    expect(paths(active, EVENTS)).toHaveLength(2);
  });

  it('polls with at most one in-flight request per source', async () => {
    const pending = deferred<Response>();
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) return json({ error: 'full' }, 503);
      if (path === CONNECTIVITY && paths(current(), CONNECTIVITY).length === 1)
        return pending.promise;
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await flush();
    await vi.advanceTimersByTimeAsync(5_000);
    expect(paths(active, CONNECTIVITY)).toHaveLength(1);
    expect(paths(active, RESOURCES).length).toBeGreaterThanOrEqual(2);
    expect(paths(active, THROUGHPUT).length).toBeGreaterThanOrEqual(2);
    pending.resolve(json(connectivityBody));
    await vi.advanceTimersByTimeAsync(5_000);
    expect(paths(active, CONNECTIVITY)).toHaveLength(2);
  });

  it('stops fallback polling only after a valid stream response', async () => {
    const healthy = controlledSSE();
    let calls = 0;
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === EVENTS) {
        calls += 1;
        return calls === 1 ? json({ error: 'full' }, 503) : healthy.response;
      }
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await flush();
    await vi.advanceTimersByTimeAsync(2_000);
    await eventually(() => expect(active?.dashboard.streaming).toBe(true));
    await vi.advanceTimersByTimeAsync(15_000);
    for (const path of GETS) expect(paths(active, path)).toHaveLength(1);
    expect(paths(active, EVENTS)).toHaveLength(2);
  });

  it('a current fallback 401 expires and aborts retries and polling', async () => {
    active = harness((request) => {
      const path = new URL(request.url).pathname;
      if (path === CONNECTIVITY || path === EVENTS) return json({ error: 'gone' }, 401);
      return bodyFor(path);
    });
    active.dashboard.start(1);
    await eventually(() => expect(active?.expire).toHaveBeenCalledTimes(1));
    await vi.advanceTimersByTimeAsync(20_000);
    expect(paths(active, EVENTS)).toHaveLength(1);
    expect(paths(active, CONNECTIVITY)).toHaveLength(1);
  });
});
