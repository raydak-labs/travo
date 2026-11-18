import { afterEach, describe, expect, it, vi } from 'vitest';
import { phraseMatches, requestFactoryReset, resetError } from './factory-reset';
import { postFactoryReset } from './gen';

vi.mock('./gen', () => ({ postFactoryReset: vi.fn() }));

afterEach(() => vi.resetAllMocks());

function reply<T>(data: T | undefined, error?: { code?: string }, status = 202) {
  return {
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  } as never;
}

describe('factory reset helpers', () => {
  it('needs the exact phrase', () => {
    expect(phraseMatches('factory reset')).toBe(true);
    for (const typed of ['', 'Factory reset', 'factory reset ', 'factory  reset']) {
      expect(phraseMatches(typed)).toBe(false);
    }
  });

  it('maps codes', () => {
    expect(resetError('operation_in_progress')).toContain('in progress');
    expect(resetError('factory_reset_pending')).toContain('already requested');
    expect(resetError('session_required')).toContain('browser session');
    expect(resetError(undefined)).toContain('Could not start');
  });

  it('sends the typed phrase and reports refusals', async () => {
    vi.mocked(postFactoryReset).mockResolvedValueOnce(reply({ status: 'ok' }));
    expect(await requestFactoryReset('factory reset')).toEqual({ ok: true });
    expect(postFactoryReset).toHaveBeenCalledWith({ body: { confirm: 'factory reset' } });
    vi.mocked(postFactoryReset).mockResolvedValueOnce(
      reply(undefined, { code: 'operation_in_progress' }, 409),
    );
    expect(await requestFactoryReset('factory reset')).toEqual({
      error: resetError('operation_in_progress'),
    });
    vi.mocked(postFactoryReset).mockRejectedValueOnce(new TypeError('offline'));
    expect(await requestFactoryReset('factory reset')).toEqual({ error: resetError(undefined) });
  });
});
