import { csrf } from './api/csrf';
import {
  getSession,
  getSetupState,
  postLogin,
  postLogout,
  postPassword,
  postSetupPassword,
  type SessionBody,
} from './api/gen';
import type { AuthState, Route } from './router';

/** Result of a mutating auth call; `code` is the daemon's stable error code, or `network`. */
export type AuthResult = { ok: true } | { ok: false; code: string };

// setTimeout stores its delay in 32 bits: longer delays fire immediately.
const MAX_TIMER_MS = 2 ** 31 - 1;

let state = $state<AuthState>('unknown');
let generation = $state(0);
let error = $state<string | null>(null);
let timer: ReturnType<typeof setTimeout> | undefined;
let returnTo: Route | null = null;

type Outcome = { response?: Response; error?: unknown };

/** The `code` of an `{error, code}` body, `network` when no response arrived at all. */
function codeOf({ response, error }: Outcome): string {
  if (!response) return 'network';
  const code = (error as { code?: unknown } | undefined)?.code;
  return typeof code === 'string' && code !== '' ? code : `http_${response.status}`;
}

function describe({ response, error }: Outcome): string {
  if (!response) return error instanceof Error ? error.message : 'network error';
  return `HTTP ${response.status}`;
}

function clearTimer() {
  clearTimeout(timer);
  timer = undefined;
}

/** Keeps the CSRF token and arms the expiry timer from the relative `expires_in` (never a server timestamp). */
function start(body: SessionBody) {
  generation += 1;
  csrf.set(body.csrf_token);
  clearTimer();
  timer = setTimeout(expire, Math.min(body.expires_in * 1000, MAX_TIMER_MS));
  state = 'authenticated';
  error = null;
}

function expire() {
  generation += 1;
  clearTimer();
  csrf.set(null);
  state = 'anonymous';
}

// Codes that mean the client's idea of the setup state is wrong: re-learn it so the guard moves on.
const STALE_STATE = new Set(['setup_required', 'already_set']);

async function finish(call: Promise<Outcome & { data?: SessionBody }>): Promise<AuthResult> {
  const out = await call;
  if (out.data) {
    start(out.data);
    return { ok: true };
  }
  const code = codeOf(out);
  if (STALE_STATE.has(code)) await session.bootstrap();
  return { ok: false, code };
}

export const session = {
  // Pages never navigate after an auth call: the state changes and App's guard effect moves on.
  get state(): AuthState {
    return state;
  },
  get error(): string | null {
    return error;
  },
  get generation(): number {
    return generation;
  },
  /** Where the user was headed before the guard sent them to login; cleared by whoever uses it. */
  get returnTo(): Route | null {
    return returnTo;
  },
  set returnTo(route: Route | null) {
    returnTo = route;
  },

  /** Learns the state from the daemon. Anything but a clear answer leaves `unknown` with `error`. */
  async bootstrap(): Promise<void> {
    const setup = await getSetupState();
    if (setup.response?.status !== 200 || !setup.data) {
      error = describe(setup);
      return;
    }
    if (!setup.data.password_set) {
      state = 'setup_required';
      error = null;
      return;
    }
    const current = await getSession();
    if (current.data) start(current.data);
    else if (current.response?.status === 401) {
      expire();
      error = null;
    } else error = describe(current);
  },

  login: (password: string): Promise<AuthResult> => finish(postLogin({ body: { password } })),
  setup: (password: string): Promise<AuthResult> =>
    finish(postSetupPassword({ body: { password } })),
  changePassword: (current: string, next: string): Promise<AuthResult> =>
    finish(postPassword({ body: { current, new: next } })),

  /** Ends the session locally whatever the daemon answered: the cookie is gone or useless either way. */
  async logout(): Promise<void> {
    const request = postLogout({ headers: { 'X-CSRF-Token': csrf.get() ?? '' } });
    expire();
    await request;
  },

  expire,
};
