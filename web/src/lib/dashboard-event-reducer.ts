import type { DashboardConnectivity, SystemInfoBody, ThroughputBody } from './api/gen';
import type { LoadStatus } from './ui/loader.svelte';

export const SOURCE_NAMES = ['connectivity', 'resources', 'throughput'] as const;
export type DashboardSourceName = (typeof SOURCE_NAMES)[number];
export type SourceData = DashboardConnectivity | SystemInfoBody | ThroughputBody;

export const sourceLabels: Record<DashboardSourceName, string> = {
  connectivity: 'Connectivity',
  resources: 'Resources',
  throughput: 'Throughput',
};

/** What one SSE event means for the dashboard state. */
export type DashboardUpdate =
  | { kind: 'ignore' }
  | { kind: 'data'; source: DashboardSourceName; data: SourceData }
  | { kind: 'unavailable'; source: DashboardSourceName };

function isSource(value: unknown): value is DashboardSourceName {
  return SOURCE_NAMES.includes(value as DashboardSourceName);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Maps one named event to an update. Unknown event names are ignored (forward compatible);
 * a known name with a malformed body throws, which the transport treats as a broken stream.
 */
export function reduceDashboardEvent(name: string, data: unknown): DashboardUpdate {
  if (name !== 'source_unavailable' && !isSource(name)) return { kind: 'ignore' };
  if (!isRecord(data)) throw new Error('Invalid event data');
  if (name === 'source_unavailable') {
    if (!isSource(data.source)) throw new Error('Invalid event source');
    return { kind: 'unavailable', source: data.source };
  }
  return { kind: 'data', source: name as DashboardSourceName, data: data as SourceData };
}

export function unavailableStatus(source: DashboardSourceName): LoadStatus {
  return { state: 'error', message: `${sourceLabels[source]} unavailable` };
}

export function errorStatus(error: unknown, fallback: string): LoadStatus {
  return {
    state: 'error',
    message: error instanceof Error && error.message !== '' ? error.message : fallback,
  };
}
