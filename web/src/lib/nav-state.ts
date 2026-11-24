import { type NavGroup, navGroup, type Route } from './router';

/** Which nav groups are open. Stored per browser; a convenience, never required. */
export type NavState = Record<NavGroup, boolean>;

export const NAV_STATE_KEY = 'nav:v1';
const VERSION = 1;

const COLLAPSED: NavState = { network: false, system: false };

/** Reads the stored value; a missing, malformed or other-version value gives the first-visit state. */
export function parseNavState(raw: string | null): NavState {
  if (raw === null) return { ...COLLAPSED };
  try {
    const value: unknown = JSON.parse(raw);
    if (typeof value !== 'object' || value === null) return { ...COLLAPSED };
    const { version, open } = value as { version?: unknown; open?: unknown };
    if (version !== VERSION || typeof open !== 'object' || open === null) return { ...COLLAPSED };
    const groups = open as Record<string, unknown>;
    return { network: groups.network === true, system: groups.system === true };
  } catch {
    return { ...COLLAPSED };
  }
}

export function serializeNavState(state: NavState): string {
  return JSON.stringify({ version: VERSION, open: state });
}

/** Opens the group that holds the current route, so its highlighted child is visible. */
export function withRoute(state: NavState, route: Route): NavState {
  const group = navGroup(route);
  return group === null || state[group] ? state : { ...state, [group]: true };
}

/** localStorage can be absent or throw (private mode, blocked site data): fall back to collapsed. */
export function loadNavState(storage: Pick<Storage, 'getItem'> | undefined): NavState {
  try {
    return parseNavState(storage?.getItem(NAV_STATE_KEY) ?? null);
  } catch {
    return { ...COLLAPSED };
  }
}

export function saveNavState(storage: Pick<Storage, 'setItem'> | undefined, state: NavState): void {
  try {
    storage?.setItem(NAV_STATE_KEY, serializeNavState(state));
  } catch {
    // Not persisting only costs the remembered state.
  }
}
