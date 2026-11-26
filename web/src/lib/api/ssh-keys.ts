import { m } from '../paraglide/messages.js';
import { client } from './client';
import { addSshKey, deleteSshKey, getSshKeys, type KeyList, type PublicKey } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchSSHKeys(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<KeyList> {
  const { data, response } = await getSshKeys({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.ssh_unavailable(), response?.status);
  return data;
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function sshKeyError(code: string | undefined): string {
  switch (code) {
    case 'ssh_key_invalid':
    case 'invalid_request':
      return m.ssh_err_invalid();
    case 'ssh_key_exists':
      return m.ssh_err_exists();
    case 'ssh_key_limit':
      return m.ssh_err_limit();
    case 'not_found':
      return m.ssh_err_not_found();
    default:
      return m.ssh_err_failed();
  }
}

/** A pasted key: one line, trimmed. More than one line is refused here, before the router. */
export function pastedKey(text: string): string | undefined {
  const line = text.trim();
  return line !== '' && !/[\r\n]/.test(line) ? line : undefined;
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function addSSHKey(line: string): Promise<{ key: PublicKey } | { error: string }> {
  try {
    const { data, error } = await addSshKey({ body: { key: line } });
    return data ? { key: data } : { error: sshKeyError(code(error)) };
  } catch {
    return { error: sshKeyError(undefined) };
  }
}

/** Removing a key that is already gone counts as done. */
export async function removeSSHKey(id: string): Promise<{ ok: true } | { error: string }> {
  try {
    const { data, error } = await deleteSshKey({ path: { id } });
    if (data || code(error) === 'not_found') return { ok: true };
    return { error: sshKeyError(code(error)) };
  } catch {
    return { error: sshKeyError(undefined) };
  }
}
