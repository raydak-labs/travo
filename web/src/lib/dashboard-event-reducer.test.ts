import { describe, expect, it } from 'vitest';
import { errorStatus, reduceDashboardEvent, unavailableStatus } from './dashboard-event-reducer';
import { checkEventResponse } from './dashboard-sse';

describe('reduceDashboardEvent', () => {
  it('maps a source event to its data', () => {
    const data = { interfaces: [] };
    expect(reduceDashboardEvent('throughput', data)).toEqual({
      kind: 'data',
      source: 'throughput',
      data,
    });
  });

  it('maps source_unavailable to the named source', () => {
    expect(reduceDashboardEvent('source_unavailable', { source: 'resources' })).toEqual({
      kind: 'unavailable',
      source: 'resources',
    });
  });

  it('accepts the uplinks and clients sources', () => {
    const clients = { clients: [], partial: true };
    expect(reduceDashboardEvent('clients', clients)).toEqual({
      kind: 'data',
      source: 'clients',
      data: clients,
    });
    expect(reduceDashboardEvent('source_unavailable', { source: 'uplinks' })).toEqual({
      kind: 'unavailable',
      source: 'uplinks',
    });
  });

  it('ignores unknown event names, even with odd bodies', () => {
    expect(reduceDashboardEvent('message', 'hello')).toEqual({ kind: 'ignore' });
    expect(reduceDashboardEvent('future_source', null)).toEqual({ kind: 'ignore' });
  });

  it('rejects malformed bodies of known events', () => {
    expect(() => reduceDashboardEvent('resources', [])).toThrow('Invalid event data');
    expect(() => reduceDashboardEvent('source_unavailable', { source: 'disk' })).toThrow(
      'Invalid event source',
    );
  });
});

describe('dashboard statuses', () => {
  it('names the unavailable source', () => {
    expect(unavailableStatus('connectivity')).toEqual({
      state: 'error',
      message: 'Connectivity unavailable',
    });
  });

  it('prefers the error message and falls back otherwise', () => {
    expect(errorStatus(new Error('boom'), 'x')).toEqual({ state: 'error', message: 'boom' });
    expect(errorStatus('nope', 'fallback')).toEqual({ state: 'error', message: 'fallback' });
  });
});

describe('checkEventResponse', () => {
  const stream = (status: number, type: string | null) =>
    new Response(status === 200 ? 'data: x\n\n' : null, {
      status,
      headers: type ? { 'Content-Type': type } : {},
    });

  it('accepts a 200 event stream with parameters', () => {
    expect(() => checkEventResponse(stream(200, 'Text/Event-Stream; charset=utf-8'))).not.toThrow();
  });

  it('rejects 401, other statuses and other content types', () => {
    expect(() => checkEventResponse(stream(401, null))).toThrow('Session expired');
    expect(() => checkEventResponse(stream(503, null))).toThrow('Events HTTP 503');
    expect(() => checkEventResponse(stream(200, 'application/json'))).toThrow(
      'Events content type: application/json',
    );
  });
});
