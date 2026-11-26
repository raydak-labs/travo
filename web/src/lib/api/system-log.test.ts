import { afterEach, describe, expect, it, vi } from 'vitest';
import { getSystemLog } from './gen';
import { fetchSystemLog, levelKind, logQuery, shortTime } from './system-log';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ getSystemLog: vi.fn() }));

afterEach(() => vi.resetAllMocks());

describe('system log helpers', () => {
  it('sends only narrowing parameters, clamped', () => {
    expect(logQuery({ lines: 200, filter: '  ' })).toEqual({ lines: 200 });
    expect(logQuery({ lines: 5000, level: 'warn', filter: ' dropbear ' })).toEqual({
      lines: 1000,
      level: 'warn',
      filter: 'dropbear',
    });
    expect(logQuery({ lines: 0, filter: 'x'.repeat(200) }).filter).toHaveLength(128);
  });

  it('colours levels by severity', () => {
    expect(levelKind('err')).toBe('danger');
    expect(levelKind('crit')).toBe('danger');
    expect(levelKind('warn')).toBe('warn');
    expect(levelKind('notice')).toBe('info');
    expect(levelKind('debug')).toBe('neutral');
    expect(levelKind('')).toBe('neutral');
  });

  it('shortens router-local times and keeps anything else', () => {
    expect(shortTime('2026-11-17T21:58:39')).toBe('11-17 21:58:39');
    expect(shortTime('')).toBe('');
  });

  it('fetches with the query and fails as a load error', async () => {
    vi.mocked(getSystemLog).mockResolvedValueOnce({
      data: { entries: [], truncated: false },
      request: new Request('http://localhost'),
      response: new Response(null),
    } as never);
    await fetchSystemLog({ lines: 50, level: 'err', filter: '' });
    expect(vi.mocked(getSystemLog).mock.calls[0]?.[0]?.query).toEqual({ lines: 50, level: 'err' });
    vi.mocked(getSystemLog).mockResolvedValueOnce({
      data: undefined,
      request: new Request('http://localhost'),
      response: new Response(null, { status: 503 }),
    } as never);
    await expect(fetchSystemLog({ lines: 1, filter: '' })).rejects.toMatchObject({ status: 503 });
  });
});
