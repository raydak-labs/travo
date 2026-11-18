import { m } from '../paraglide/messages.js';
import { postFactoryReset } from './gen';

/** The phrase the daemon requires, typed exactly. */
export const RESET_PHRASE = 'factory reset';

export function phraseMatches(typed: string): boolean {
  return typed === RESET_PHRASE;
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function resetError(code: string | undefined): string {
  switch (code) {
    case 'operation_in_progress':
      return m.reset_err_busy();
    case 'factory_reset_pending':
      return m.reset_err_pending();
    case 'confirmation_required':
      return m.reset_err_phrase();
    case 'session_required':
      return m.reset_err_session();
    default:
      return m.reset_err_failed();
  }
}

export async function requestFactoryReset(
  typed: string,
): Promise<{ ok: true } | { error: string }> {
  try {
    const { data, error } = await postFactoryReset({ body: { confirm: typed } });
    if (data) return { ok: true };
    return { error: resetError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: resetError(undefined) };
  }
}
