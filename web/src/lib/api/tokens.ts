import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type ApiTokenCreated,
  createApiToken,
  deleteApiToken,
  getApiTokens,
  type Token,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type TokenScope = Token['scope'];

/** Newest first: the token just created is the one people look for. */
export function sortTokens(tokens: readonly Token[]): Token[] {
  return [...tokens].sort((a, b) => b.created_at.localeCompare(a.created_at));
}

export async function fetchTokens(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getApiTokens({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.tokens_unavailable(), response?.status);
  return sortTokens(data.tokens);
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function tokenError(code: string | undefined): string {
  switch (code) {
    case 'token_limit':
      return m.tokens_err_limit();
    case 'invalid_request':
      return m.tokens_err_invalid();
    case 'not_found':
      return m.tokens_err_not_found();
    case 'session_required':
      return m.tokens_err_session();
    default:
      return m.tokens_err_failed();
  }
}

/** A trimmed name the daemon accepts (1..256 characters), or undefined. */
export function tokenName(raw: string): string | undefined {
  const name = raw.trim();
  return name.length > 0 && name.length <= 256 ? name : undefined;
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

/** Creates a token; the result carries the raw secret, which the API never returns again. */
export async function createToken(
  name: string,
  scope: TokenScope,
): Promise<{ created: ApiTokenCreated } | { error: string }> {
  try {
    const { data, error } = await createApiToken({ body: { name, scope } });
    return data ? { created: data } : { error: tokenError(code(error)) };
  } catch {
    return { error: tokenError(undefined) };
  }
}

/** Revokes a token. An unknown id is already gone, which is what the caller wanted. */
export async function revokeToken(id: string): Promise<{ ok: true } | { error: string }> {
  try {
    const { data, error } = await deleteApiToken({ path: { id } });
    if (data || code(error) === 'not_found') return { ok: true };
    return { error: tokenError(code(error)) };
  } catch {
    return { error: tokenError(undefined) };
  }
}
