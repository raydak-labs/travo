// In-memory CSRF token. Never persisted: the daemon hands it out at login/session start.
let token: string | null = null;

export const csrf = {
  get: (): string | null => token,
  set: (value: string | null): void => {
    token = value;
  },
};

const SAFE = new Set(['GET', 'HEAD', 'OPTIONS']);

/** Request interceptor for the generated client: adds X-CSRF-Token on unsafe methods only. */
export function csrfInterceptor(request: Request): Request {
  const value = csrf.get();
  if (value && !SAFE.has(request.method.toUpperCase())) {
    request.headers.set('X-CSRF-Token', value);
  }
  return request;
}
