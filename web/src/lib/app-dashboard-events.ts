import type { dashboardEvents } from './dashboard-events.svelte';

export type DashboardEventOwner = Pick<typeof dashboardEvents, 'start' | 'stop' | 'clear'>;

/** Owns one authenticated generation; the returned cleanup cannot stop its replacement. */
export function ownDashboardEvents(
  authenticated: boolean,
  generation: number,
  events: DashboardEventOwner,
): (() => void) | undefined {
  if (!authenticated) {
    events.clear();
    return;
  }
  events.start(generation);
  return () => events.stop(generation);
}

/** A persisted page keeps its session and values, but its aborted stream needs a new owner. */
export function restoreDashboardEvents(
  persisted: boolean,
  authenticated: boolean,
  generation: number,
  events: DashboardEventOwner,
): void {
  if (persisted && authenticated) events.start(generation);
}
