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

/** Clears auth state and redirects to the login page. Exported for testability. */
export function handleUnauthorized(): void {
  clearToken();
  if (typeof window !== 'undefined') {
    window.location.assign('/login');
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

function buildHeaders(body: unknown): Record<string, string> {
  const headers: Record<string, string> = {};
  // FormData must set its own multipart boundary, and bodyless requests
  // (every GET/DELETE) must not claim a JSON content type at all.
  if (body !== undefined && !(typeof FormData !== 'undefined' && body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }
  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

function bodyToRequestBody(body: unknown): BodyInit | undefined {
  if (body === undefined) return undefined;
  if (typeof FormData !== 'undefined' && body instanceof FormData) return body;
  return JSON.stringify(body);
}

/**
 * Performs a request and normalises failures: a 401 (except on the login
 * endpoint) clears the session, and every non-2xx becomes an `ApiError` whose
 * message survives a non-JSON body.
 */
async function send(path: string, method: string, body?: unknown, signal?: AbortSignal) {
  const response = await fetch(path, {
    method,
    headers: buildHeaders(body),
    body: bodyToRequestBody(body),
    signal,
  });

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

function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  return send(path, method, body).then((response) => response.json() as Promise<T>);
}

/** GET that returns a binary body (e.g. a backup archive). */
function requestBlob(path: string): Promise<Blob> {
  return send(path, 'GET').then((response) => response.blob());
}

export const apiClient = {
  get<T>(path: string): Promise<T> {
    return request<T>('GET', path);
  },
  getBlob(path: string): Promise<Blob> {
    return requestBlob(path);
  },
  post<T>(path: string, body?: unknown): Promise<T> {
    return request<T>('POST', path, body);
  },
  /** POST with a `multipart/form-data` body (backup restore, firmware upload). */
  postForm<T>(path: string, formData: FormData): Promise<T> {
    return request<T>('POST', path, formData);
  },
  put<T>(path: string, body?: unknown): Promise<T> {
    return request<T>('PUT', path, body);
  },
  del<T>(path: string): Promise<T> {
    return request<T>('DELETE', path);
  },
};

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
  const response = await send(path, 'POST', undefined, signal);

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
