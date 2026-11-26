import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApiToken, deleteApiToken, getApiTokens, type Token } from './gen';
import { createToken, fetchTokens, revokeToken, sortTokens, tokenError, tokenName } from './tokens';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({
  getApiTokens: vi.fn(),
  createApiToken: vi.fn(),
  deleteApiToken: vi.fn(),
}));

afterEach(() => vi.resetAllMocks());

const older: Token = {
  id: '0'.repeat(32),
  name: 'Home Assistant',
  scope: 'read',
  created_at: '2026-11-17T08:00:00Z',
};
const newer: Token = {
  ...older,
  id: '1'.repeat(32),
  name: 'backup',
  created_at: '2026-11-18T09:30:00Z',
};

function reply<T>(data: T | undefined, error?: { code?: string }, status = 200) {
  return {
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  } as never;
}

describe('API token helpers', () => {
  it('lists newest first', async () => {
    vi.mocked(getApiTokens).mockResolvedValue(reply({ tokens: [older, newer] }));
    expect((await fetchTokens()).map((t) => t.name)).toEqual(['backup', 'Home Assistant']);
    expect(sortTokens([older, newer])[0]).toBe(newer);
  });

  it('turns a failed list into a load error with the status', async () => {
    vi.mocked(getApiTokens).mockResolvedValue(
      reply(undefined, { code: 'tokens_unavailable' }, 503),
    );
    await expect(fetchTokens()).rejects.toMatchObject({
      message: 'API tokens are unavailable',
      status: 503,
    });
  });

  it.each([
    ['token_limit', 'The token limit (16) is reached. Revoke a token first.'],
    ['invalid_request', 'Enter a name of 1 to 256 characters.'],
    ['not_found', 'This token no longer exists.'],
    ['session_required', 'Tokens can only be managed from a signed-in browser session.'],
    ['tokens_unavailable', 'Could not change API tokens. Try again.'],
    [undefined, 'Could not change API tokens. Try again.'],
  ])('maps %s to its message', (code, text) => {
    expect(tokenError(code)).toBe(text);
  });

  it('accepts names the daemon accepts, trimmed', () => {
    expect(tokenName('  HA  ')).toBe('HA');
    expect(tokenName('   ')).toBeUndefined();
    expect(tokenName('x'.repeat(256))).toHaveLength(256);
    expect(tokenName('x'.repeat(257))).toBeUndefined();
  });

  it('returns the one-time secret on create', async () => {
    const created = { ...newer, token: 'tgt_secret' };
    vi.mocked(createApiToken).mockResolvedValue(reply(created));
    expect(await createToken('backup', 'admin')).toEqual({ created });
    expect(createApiToken).toHaveBeenCalledWith({ body: { name: 'backup', scope: 'admin' } });
  });

  it('maps a create failure by code and a transport failure to the generic message', async () => {
    vi.mocked(createApiToken).mockResolvedValueOnce(reply(undefined, { code: 'token_limit' }, 409));
    expect(await createToken('x', 'read')).toEqual({ error: tokenError('token_limit') });
    vi.mocked(createApiToken).mockRejectedValueOnce(new TypeError('offline'));
    expect(await createToken('x', 'read')).toEqual({ error: tokenError(undefined) });
  });

  it('treats revoking an unknown token as done', async () => {
    vi.mocked(deleteApiToken).mockResolvedValueOnce(reply({ status: 'ok' }));
    expect(await revokeToken(older.id)).toEqual({ ok: true });
    expect(deleteApiToken).toHaveBeenCalledWith({ path: { id: older.id } });
    vi.mocked(deleteApiToken).mockResolvedValueOnce(reply(undefined, { code: 'not_found' }, 404));
    expect(await revokeToken(older.id)).toEqual({ ok: true });
    vi.mocked(deleteApiToken).mockResolvedValueOnce(
      reply(undefined, { code: 'tokens_unavailable' }, 503),
    );
    expect(await revokeToken(older.id)).toEqual({ error: tokenError(undefined) });
  });
});
