import { describe, expect, it, vi } from 'vitest';
import { ownDashboardEvents, restoreDashboardEvents } from './app-dashboard-events';

function eventOwner() {
  return { start: vi.fn(), stop: vi.fn(), clear: vi.fn() };
}

describe('App dashboard session ownership', () => {
  it('starts on login, replaces a changed generation, and stops and clears on logout', () => {
    const events = eventOwner();

    let cleanup = ownDashboardEvents(false, 0, events);
    expect(events.start).not.toHaveBeenCalled();
    expect(events.clear).toHaveBeenCalledOnce();

    cleanup?.();
    cleanup = ownDashboardEvents(true, 1, events);
    expect(events.start).toHaveBeenCalledOnce();
    expect(events.start).toHaveBeenLastCalledWith(1);

    // App runs the previous effect cleanup before a generation-keyed rerun.
    cleanup?.();
    cleanup = ownDashboardEvents(true, 2, events);
    expect(events.stop).toHaveBeenCalledOnce();
    expect(events.stop).toHaveBeenLastCalledWith(1);
    expect(events.start).toHaveBeenCalledTimes(2);
    expect(events.start).toHaveBeenLastCalledWith(2);

    cleanup?.();
    cleanup = ownDashboardEvents(false, 3, events);
    expect(events.stop).toHaveBeenCalledTimes(2);
    expect(events.stop).toHaveBeenLastCalledWith(2);
    expect(events.clear).toHaveBeenCalledTimes(2);
    expect(events.start).toHaveBeenCalledTimes(2);
    expect(cleanup).toBeUndefined();
  });
});

describe('App dashboard bfcache ownership', () => {
  it('restarts the current generation only for an authenticated persisted restore', () => {
    const events = eventOwner();

    restoreDashboardEvents(false, true, 4, events);
    restoreDashboardEvents(true, false, 4, events);
    expect(events.start).not.toHaveBeenCalled();

    restoreDashboardEvents(true, true, 4, events);
    expect(events.start).toHaveBeenCalledOnce();
    expect(events.start).toHaveBeenCalledWith(4);
  });
});
