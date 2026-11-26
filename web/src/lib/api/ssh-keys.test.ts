import { afterEach, describe, expect, it, vi } from 'vitest';
import { addSshKey, deleteSshKey, getSshKeys } from './gen';
import { addSSHKey, fetchSSHKeys, pastedKey, removeSSHKey, sshKeyError } from './ssh-keys';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ getSshKeys: vi.fn(), addSshKey: vi.fn(), deleteSshKey: vi.fn() }));

afterEach(() => vi.resetAllMocks());

const key = {
  id: 'a'.repeat(64),
  type: 'ssh-ed25519',
  comment: 'laptop',
  fingerprint: 'SHA256:wDQ6rSZgjr4cNqg9V83faMCsq8u5Q3JFikiOxv5Rw84',
};

function reply<T>(data: T | undefined, error?: { code?: string }, status = 200) {
  return {
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  } as never;
}

describe('SSH key helpers', () => {
  it('takes one trimmed line and refuses several', () => {
    expect(pastedKey('  ssh-ed25519 AAAA x \n')).toBe('ssh-ed25519 AAAA x');
    expect(pastedKey('ssh-ed25519 A\nssh-ed25519 B')).toBeUndefined();
    expect(pastedKey('   ')).toBeUndefined();
  });

  it.each([
    ['ssh_key_invalid', 'Paste exactly one public key line'],
    ['invalid_request', 'Paste exactly one public key line'],
    ['ssh_key_exists', 'already authorized'],
    ['ssh_key_limit', 'limit (32)'],
    ['not_found', 'no longer exists'],
    [undefined, 'Could not change'],
  ])('maps %s', (code, text) => {
    expect(sshKeyError(code)).toContain(text);
  });

  it('lists, and turns a failed list into a load error', async () => {
    vi.mocked(getSshKeys).mockResolvedValueOnce(reply({ keys: [key], unmanaged: 1 }));
    expect(await fetchSSHKeys()).toEqual({ keys: [key], unmanaged: 1 });
    vi.mocked(getSshKeys).mockResolvedValueOnce(reply(undefined, undefined, 503));
    await expect(fetchSSHKeys()).rejects.toMatchObject({ status: 503 });
  });

  it('adds and maps failures by code', async () => {
    vi.mocked(addSshKey).mockResolvedValueOnce(reply(key));
    expect(await addSSHKey('ssh-ed25519 AAAA laptop')).toEqual({ key });
    vi.mocked(addSshKey).mockResolvedValueOnce(reply(undefined, { code: 'ssh_key_exists' }, 409));
    expect(await addSSHKey('x')).toEqual({ error: sshKeyError('ssh_key_exists') });
    vi.mocked(addSshKey).mockRejectedValueOnce(new TypeError('offline'));
    expect(await addSSHKey('x')).toEqual({ error: sshKeyError(undefined) });
  });

  it('treats removing a gone key as done', async () => {
    vi.mocked(deleteSshKey).mockResolvedValueOnce(reply(undefined, { code: 'not_found' }, 404));
    expect(await removeSSHKey(key.id)).toEqual({ ok: true });
    vi.mocked(deleteSshKey).mockResolvedValueOnce(reply(undefined, { code: 'x' }, 503));
    expect(await removeSSHKey(key.id)).toEqual({ error: sshKeyError(undefined) });
  });
});
