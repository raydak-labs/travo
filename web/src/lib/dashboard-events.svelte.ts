import { scopedAPIClient } from './api/client';
import { fetchConnectivity } from './api/connectivity';
import type { DashboardConnectivity, SystemInfoBody, ThroughputBody } from './api/gen';
import type { Client } from './api/gen/client';
import { APIRequestError } from './api/request';
import { fetchSystemInfo } from './api/system';
import { fetchThroughput } from './api/throughput';
import {
  type DashboardSourceName,
  errorStatus,
  reduceDashboardEvent,
  SOURCE_NAMES,
  type SourceData,
  sourceLabels,
  unavailableStatus,
} from './dashboard-event-reducer';
import { readEventStream } from './dashboard-sse';
import { session } from './session.svelte';
import type { LoadStatus } from './ui/loader.svelte';

export type { DashboardSourceName } from './dashboard-event-reducer';
export const DASHBOARD_RECONNECT_MS = 2_000;
export const DASHBOARD_FALLBACK_MS = 5_000;

type SourceState = { status: LoadStatus; data?: SourceData };
type Reader = (signal: AbortSignal | undefined, client: Client) => Promise<SourceData>;

const readers: Record<DashboardSourceName, Reader> = {
  connectivity: fetchConnectivity,
  resources: fetchSystemInfo,
  throughput: fetchThroughput,
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
    const update = reduceDashboardEvent(name, data);
    if (update.kind === 'ignore') return;
    // A pushed value supersedes any seed or fallback read still in flight.
    owner.versions[update.source] += 1;
    if (update.kind === 'unavailable') {
      states[update.source].status = unavailableStatus(update.source);
      return;
    }
    states[update.source].data = update.data;
    states[update.source].status = { state: 'ready' };
  }

  function connect(owner: Ownership): Promise<'retry' | 'unauthorized'> {
    return readEventStream({
      client: owner.client,
      signal: owner.controller.signal,
      onOpen: () => {
        if (!isCurrent(owner)) return;
        owner.connected = true;
        streaming = true;
        stopFallback(owner);
      },
      onEvent: (name, data) => applyEvent(owner, name, data),
    });
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
