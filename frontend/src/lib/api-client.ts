import { currentRelativeLocation } from '@/lib/auth-redirect';

const TOKEN_KEY = 'openwrt-auth-token';

/**
 * Fired on `window` whenever the auth token is set or cleared so that
 * long-lived singletons (the WebSocket provider) can react without polling
 * storage on every render.
 */
export const TOKEN_CHANGE_EVENT = 'openwrt-travel-gui:token-change';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY) ?? sessionStorage.getItem(TOKEN_KEY);
}

function notifyTokenChange(): void {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(TOKEN_CHANGE_EVENT));
  }
}

/**
 * Whether the current session is persisted across browser restarts.
 *
 * A token-change flow (e.g. the replacement token returned by a password
 * change) must preserve the user's original choice: a session started with
 * "don't remember me" lives in sessionStorage, and re-storing it with the
 * default would silently promote it to localStorage so it outlived the tab.
 */
export function isTokenRemembered(): boolean {
  return localStorage.getItem(TOKEN_KEY) !== null;
}

export function setToken(token: string, remember = true): void {
  if (remember) {
    localStorage.setItem(TOKEN_KEY, token);
    sessionStorage.removeItem(TOKEN_KEY);
  } else {
    sessionStorage.setItem(TOKEN_KEY, token);
    localStorage.removeItem(TOKEN_KEY);
  }
  notifyTokenChange();
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
  sessionStorage.removeItem(TOKEN_KEY);
  notifyTokenChange();
}

/**
 * Clears auth state and redirects to the login page, carrying the current
 * location so the user lands back where they were. Exported for testability.
 */
export function handleUnauthorized(): void {
  clearToken();
  if (typeof window !== 'undefined') {
    window.location.assign(`/login?redirect=${encodeURIComponent(currentRelativeLocation())}`);
  }
}

/** An HTTP error response, carrying the status code for callers that must branch on it. */
export class ApiError extends Error {
  readonly status: number;
  /**
   * The server's machine-readable error code, when it sent one.
   *
   * Callers must branch on this rather than on the message: the wireless
   * lockout refusal rewords its message whenever the remedy gets clearer, and a
   * client that matched the text would stop recognising it.
   */
  readonly code?: string;

  constructor(status: number, message: string, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

/**
 * Extracts the human-readable message and the optional machine-readable code
 * from a failed response.
 *
 * A router can answer with an HTML error page (502 from a proxy, 401 from a
 * captive portal, …), so the JSON body is parsed defensively and the HTTP
 * status line is used as the fallback instead of a `SyntaxError`.
 */
export async function errorFromResponse(
  response: Response,
  fallback: string,
): Promise<{ message: string; code?: string }> {
  try {
    const body = (await response.json()) as { error?: unknown; code?: unknown } | null;
    if (body && typeof body.error === 'string' && body.error.length > 0) {
      const code = typeof body.code === 'string' ? body.code : undefined;
      return { message: body.error, code };
    }
  } catch {
    // Non-JSON body (HTML error page, empty body) — keep the fallback.
  }
  return { message: fallback };
}

/**
 * Extracts a human-readable error message from a failed response.
 *
 * A router can answer with an HTML error page (502 from a proxy, 401 from a
 * captive portal, …), so the JSON body is parsed defensively and the HTTP
 * status line is used as the fallback instead of a `SyntaxError`.
 */
export async function errorMessageFromResponse(
  response: Response,
  fallback: string,
): Promise<string> {
  return (await errorFromResponse(response, fallback)).message;
}

/**
 * Performs a request with the session token and normalises failures: a 401
 * (except on the login endpoint) clears the session, and every non-2xx becomes
 * an `ApiError` whose message survives a non-JSON body.
 *
 * Pass `init.signal` on reads so React Query can cancel them; a fetch left
 * running costs RAM on a 128 MB router.
 */
async function send(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const response = await fetch(path, { ...init, headers });
  if (!response.ok) {
    if (response.status === 401 && !path.endsWith('/auth/login')) {
      handleUnauthorized();
    }
    const { message, code } = await errorFromResponse(
      response,
      `Request failed with status ${response.status}`,
    );
    throw new ApiError(response.status, message, code);
  }
  return response;
}

/**
 * The HTTP client behind the generated request functions (`@/api/endpoints`,
 * see orval.config.mjs): a JSON body in, the decoded JSON body out.
 */
export async function apiFetch<T>(url: string, init: RequestInit): Promise<T> {
  const response = await send(url, init);
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

/** GET that returns a binary body (e.g. a backup archive). */
export function getBlob(path: string, signal?: AbortSignal): Promise<Blob> {
  return send(path, { signal }).then((response) => response.blob());
}

/** NDJSON stream event from the backend. */
export interface StreamEvent {
  type: 'log' | 'done' | 'error';
  data?: string;
}

/**
 * Makes a POST request that returns an NDJSON stream.
 * Calls onEvent for each parsed event. Resolves when the stream ends.
 *
 * Pass `signal` to abort an in-flight stream (e.g. when a log dialog closes);
 * the reader is cancelled, no further events fire and the promise settles.
 */
export async function streamRequest(
  path: string,
  onEvent: (event: StreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const response = await send(path, { method: 'POST', signal });

  const reader = response.body?.getReader();
  if (!reader) throw new Error('No response body');

  // `fetch` only rejects the request while it is still in flight; once the body
  // is streaming, the signal must cancel the reader explicitly.
  const onAbort = () => {
    void reader.cancel().catch(() => {});
  };
  if (signal?.aborted) onAbort();
  signal?.addEventListener('abort', onAbort, { once: true });

  const decoder = new TextDecoder();
  let buffer = '';

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';

      for (const line of lines) {
        emitStreamLine(onEvent, line);
      }
    }

    // Flush the decoder: a multi-byte character can be split across the last
    // chunk boundary and would otherwise be silently dropped.
    buffer += decoder.decode();

    // Process any remaining buffer
    emitStreamLine(onEvent, buffer);
  } catch (error) {
    // Release the socket when the caller aborts; the reader is done either way.
    await reader.cancel().catch(() => {});
    throw error;
  } finally {
    signal?.removeEventListener('abort', onAbort);
  }
}

function emitStreamLine(onEvent: (event: StreamEvent) => void, line: string): void {
  const trimmed = line.trim();
  if (!trimmed) return;
  try {
    onEvent(JSON.parse(trimmed) as StreamEvent);
  } catch {
    // skip malformed lines
  }
}
