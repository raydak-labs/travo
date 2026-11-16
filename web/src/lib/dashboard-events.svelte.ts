import { scopedAPIClient } from './api/client';
import { fetchConnectivity } from './api/connectivity';
import type {
  DashboardConnectivity,
  GetDashboardEventsData,
  SystemInfoBody,
  ThroughputBody,
} from './api/gen';
import type { Client } from './api/gen/client';
import { createSseClient } from './api/gen/core/serverSentEvents.gen';
import { APIRequestError } from './api/request';
import { fetchSystemInfo } from './api/system';
import { fetchThroughput } from './api/throughput';
import { normalizeEventBody } from './dashboard-sse';
import { session } from './session.svelte';
import type { LoadStatus } from './ui/loader.svelte';

export const DASHBOARD_RECONNECT_MS = 2_000;
export const DASHBOARD_FALLBACK_MS = 5_000;
const EVENTS_URL = '/api/v1/dashboard/events' satisfies GetDashboardEventsData['url'];
const SOURCE_NAMES = ['connectivity', 'resources', 'throughput'] as const;

export type DashboardSourceName = (typeof SOURCE_NAMES)[number];
type SourceData = DashboardConnectivity | SystemInfoBody | ThroughputBody;
type SourceState = { status: LoadStatus; data?: SourceData };
type Reader = (signal: AbortSignal | undefined, client: Client) => Promise<SourceData>;

const readers: Record<DashboardSourceName, Reader> = {
  connectivity: fetchConnectivity,
  resources: fetchSystemInfo,
  throughput: fetchThroughput,
};

const sourceLabels: Record<DashboardSourceName, string> = {
  connectivity: 'Connectivity',
  resources: 'Resources',
  throughput: 'Throughput',
};

type Ownership = {
  generation: number;
  controller: AbortController;
  client: Client;
  connected: boolean;
  fallback?: ReturnType<typeof setInterval>;
  retry?: ReturnType<typeof setTimeout>;
  inFlight: Record<DashboardSourceName, boolean>;
  versions: Record<DashboardSourceName, number>;
};

export type DashboardEventsOptions = {
  client?: () => Client;
  generation?: () => number;
  expire?: () => void;
  reconnectDelay?: number;
  fallbackInterval?: number;
};

function freshState(): SourceState {
  return { status: { state: 'loading' } };
}

function errorStatus(error: unknown, fallback: string): LoadStatus {
  return {
    state: 'error',
    message: error instanceof Error && error.message !== '' ? error.message : fallback,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** Owns the authenticated shell's one dashboard stream and its disconnected fallback. */
export function createDashboardEvents(options: DashboardEventsOptions = {}) {
  const clientFactory = options.client ?? scopedAPIClient;
  const currentGeneration = options.generation ?? (() => session.generation);
  const expireSession = options.expire ?? (() => session.expire());
  const reconnectDelay = options.reconnectDelay ?? DASHBOARD_RECONNECT_MS;
  const fallbackInterval = options.fallbackInterval ?? DASHBOARD_FALLBACK_MS;
  const connectivity = $state(freshState());
  const resources = $state(freshState());
  const throughput = $state(freshState());
  const states: Record<DashboardSourceName, SourceState> = {
    connectivity,
    resources,
    throughput,
  };
  let streaming = $state(false);
  let active: Ownership | undefined;

  function isCurrent(owner: Ownership): boolean {
    return active === owner && currentGeneration() === owner.generation;
  }

  function stopFallback(owner: Ownership): void {
    clearInterval(owner.fallback);
    owner.fallback = undefined;
  }

  function stop(generation?: number): void {
    const owner = active;
    if (!owner || (generation !== undefined && owner.generation !== generation)) return;
    active = undefined;
    owner.controller.abort();
    stopFallback(owner);
    clearTimeout(owner.retry);
    owner.retry = undefined;
    streaming = false;
  }

  function expire(owner: Ownership): void {
    if (!isCurrent(owner)) return;
    stop(owner.generation);
    expireSession();
  }

  function view<T>(source: DashboardSourceName) {
    return {
      get data(): T | undefined {
        return states[source].data as T | undefined;
      },
      get status(): LoadStatus {
        return states[source].status;
      },
      retry: () => {
        if (active) void readSource(active, source);
      },
    };
  }

  function startFallback(owner: Ownership): void {
    if (owner.fallback !== undefined || !isCurrent(owner)) return;
    owner.fallback = setInterval(() => {
      if (!isCurrent(owner) || owner.connected) return;
      for (const source of SOURCE_NAMES) void readSource(owner, source);
    }, fallbackInterval);
  }

  async function readSource(owner: Ownership, source: DashboardSourceName): Promise<void> {
    if (!isCurrent(owner) || owner.inFlight[source]) return;
    owner.inFlight[source] = true;
    const version = owner.versions[source];
    try {
      const data = await readers[source](owner.controller.signal, owner.client);
      if (!isCurrent(owner) || owner.versions[source] !== version) return;
      states[source].data = data;
      states[source].status = { state: 'ready' };
    } catch (error) {
      if (!isCurrent(owner)) return;
      if (error instanceof APIRequestError && error.status === 401) {
        expire(owner);
        return;
      }
      if (owner.versions[source] === version) {
        states[source].status = errorStatus(error, `${sourceLabels[source]} unavailable`);
      }
    } finally {
      owner.inFlight[source] = false;
    }
  }

  function applyEvent(owner: Ownership, name: string, data: unknown): void {
    if (!isCurrent(owner)) return;
    if (name !== 'source_unavailable' && !SOURCE_NAMES.includes(name as DashboardSourceName))
      return;
    if (!isRecord(data)) throw new Error('Invalid event data');
    if (name === 'source_unavailable') {
      const source = data.source as DashboardSourceName;
      if (!SOURCE_NAMES.includes(source)) throw new Error('Invalid event source');
      owner.versions[source] += 1;
      states[source].status = {
        state: 'error',
        message: `${sourceLabels[source]} unavailable`,
      };
      return;
    }
    const source = name as DashboardSourceName;
    owner.versions[source] += 1;
    states[source].data = data as SourceData;
    states[source].status = { state: 'ready' };
  }

  async function connect(owner: Ownership): Promise<'retry' | 'unauthorized'> {
    const transport = owner.client.getConfig().fetch ?? globalThis.fetch;
    let failure: unknown;
    const { stream } = createSseClient({
      url: owner.client.buildUrl({ url: EVENTS_URL }),
      credentials: 'same-origin',
      signal: owner.controller.signal,
      sseMaxRetryAttempts: 1, // reconnect below, with fresh ownership checks and no resume ID
      fetch: async (request) => {
        owner.controller.signal.throwIfAborted();
        const response = await transport(request);
        if (owner.controller.signal.aborted) {
          await response.body?.cancel();
          owner.controller.signal.throwIfAborted();
        }
        if (response.status === 401) throw new APIRequestError('Session expired', 401);
        if (response.status !== 200) throw new Error(`Events HTTP ${response.status}`);
        const contentType = response.headers
          .get('Content-Type')
          ?.split(';', 1)[0]
          ?.trim()
          .toLowerCase();
        if (contentType !== 'text/event-stream') {
          throw new Error(`Events content type: ${contentType ?? 'missing'}`);
        }
        if (!response.body) throw new Error('Events response has no body');
        if (isCurrent(owner)) {
          owner.connected = true;
          streaming = true;
          stopFallback(owner);
        }
        return new Response(normalizeEventBody(response.body), response);
      },
      onSseError: (error) => {
        failure = error;
      },
      onSseEvent: (event) => applyEvent(owner, event.event ?? 'message', event.data),
    });
    for await (const event of stream) void event;
    return failure instanceof APIRequestError && failure.status === 401 ? 'unauthorized' : 'retry';
  }

  function retryIn(owner: Ownership): Promise<boolean> {
    if (!isCurrent(owner)) return Promise.resolve(false);
    return new Promise((resolve) => {
      const finish = (retry: boolean) => {
        clearTimeout(owner.retry);
        owner.retry = undefined;
        owner.controller.signal.removeEventListener('abort', aborted);
        resolve(retry);
      };
      const aborted = () => finish(false);
      owner.retry = setTimeout(() => finish(isCurrent(owner)), reconnectDelay);
      owner.controller.signal.addEventListener('abort', aborted, { once: true });
    });
  }

  async function streamLoop(owner: Ownership): Promise<void> {
    while (isCurrent(owner)) {
      let outcome: 'retry' | 'unauthorized' = 'retry';
      let failure: unknown;
      try {
        outcome = await connect(owner);
      } catch (error) {
        failure = error;
      }
      if (!isCurrent(owner)) return;
      owner.connected = false;
      streaming = false;
      if (outcome === 'unauthorized') {
        expire(owner);
        return;
      }
      for (const source of SOURCE_NAMES) {
        states[source].status = errorStatus(failure, 'Live updates disconnected');
      }
      startFallback(owner);
      if (!(await retryIn(owner))) return;
    }
  }

  function start(generation: number): void {
    if (active?.generation === generation) return;
    stop();
    // A restarted owner has no fresh data yet; retained values are stale until a seed or event arrives.
    for (const source of SOURCE_NAMES) states[source].status = { state: 'loading' };
    const owner: Ownership = {
      generation,
      controller: new AbortController(),
      client: clientFactory(),
      connected: false,
      inFlight: { connectivity: false, resources: false, throughput: false },
      versions: { connectivity: 0, resources: 0, throughput: 0 },
    };
    active = owner;
    startFallback(owner);
    for (const source of SOURCE_NAMES) void readSource(owner, source);
    void streamLoop(owner);
  }

  return {
    connectivity: view<DashboardConnectivity>('connectivity'),
    resources: view<SystemInfoBody>('resources'),
    throughput: view<ThroughputBody>('throughput'),
    get streaming(): boolean {
      return streaming;
    },
    start,
    stop,
    clear: () => {
      stop();
      for (const source of SOURCE_NAMES) {
        states[source].data = undefined;
        states[source].status = { state: 'loading' };
      }
    },
  };
}

export const dashboardEvents = createDashboardEvents();
