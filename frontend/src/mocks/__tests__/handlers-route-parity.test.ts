import { describe, it, expect } from 'vitest';
import { API_ROUTES } from '@shared/index';
import { handlers } from '../handlers';

type RouteTree = { readonly [key: string]: string | RouteTree };

/** Every leaf of the route table, as "dotted.path" -> path. */
function collectLeaves(tree: RouteTree, prefix = ''): Map<string, string> {
  const leaves = new Map<string, string>();
  for (const [key, value] of Object.entries(tree)) {
    const dotted = prefix ? `${prefix}.${key}` : key;
    if (typeof value === 'string') {
      leaves.set(dotted, value);
    } else {
      for (const [nested, path] of collectLeaves(value, dotted)) {
        leaves.set(nested, path);
      }
    }
  }
  return leaves;
}

type MockHandler = { info?: { method?: string; path?: unknown } };

function handlerPaths(): { method: string; path: string }[] {
  return (handlers as unknown as MockHandler[]).map((handler) => ({
    method: String(handler.info?.method ?? 'GET'),
    path: String(handler.info?.path),
  }));
}

/**
 * A handler path is anchored on a route-table leaf when it *is* that leaf, or
 * when it extends it with parameter segments only (`/wifi/saved/:section`).
 * A trailing literal makes it a route the table does not describe.
 */
function anchorLeaf(path: string, leaves: Set<string>): string | null {
  const segments = path.split('/');
  for (let take = segments.length; take > 0; take--) {
    const candidate = segments.slice(0, take).join('/');
    if (!leaves.has(candidate)) continue;
    const tail = segments.slice(take);
    return tail.every((s) => s.startsWith(':') || s === '*') ? candidate : null;
  }
  return null;
}

/**
 * Routes the backend serves but shared/src/api/routes.ts does not describe, so
 * handlers.ts has to spell them out. Keep this list empty-by-default: an entry
 * needs a pointer to the backend route, and shared/ must gain the constant.
 */
const ROUTE_TABLE_GAPS: readonly string[] = [
  // backend/internal/api/router.go:200 POST /vpn/wireguard/profiles/:id/activate
  'POST /api/v1/vpn/wireguard/profiles/:id/activate',
];

describe('mock handler route parity', () => {
  const leaves = collectLeaves(API_ROUTES as unknown as RouteTree);
  const leafPaths = [...leaves.values()];
  const paths = handlerPaths();

  it('recurses into nested route groups', () => {
    // Guards the walk itself: if recursion stopped, only the top level would be
    // compared and the whole suite would pass on a partial route table.
    const nested = [...leaves.keys()].filter((key) => key.includes('.'));
    expect(nested.length).toBeGreaterThan(0);
    expect(leaves.size).toBeGreaterThan(Object.keys(API_ROUTES).length);
  });

  it('declares no duplicate method/path handlers', () => {
    // A second handler for the same method+path is dead: MSW takes the first.
    const seen = new Map<string, number>();
    for (const handler of paths) {
      const key = `${handler.method} ${handler.path}`;
      seen.set(key, (seen.get(key) ?? 0) + 1);
    }
    expect([...seen.entries()].filter(([, n]) => n > 1)).toEqual([]);
  });

  it.each([...leaves.entries()])('has a handler for %s (%s)', (_name, path) => {
    // Exact match only. A prefix match would let `/wifi/saved/:section` vouch
    // for `/wifi/saved`, so deleting the plain handler would go unnoticed.
    const covered = paths.some((handler) => handler.path === path);
    expect(
      covered,
      `No MSW handler serves ${path} exactly. Add one to src/mocks/handlers.ts: ` +
        'an unhandled route is bypassed to the network in dev and any test ' +
        'asserting against it is non-deterministic.',
    ).toBe(true);
  });

  it.each(paths.map((handler, i) => [i, handler] as const))(
    'handler %i is anchored on a route-table leaf',
    (_i, handler) => {
      const label = `${handler.method} ${handler.path}`;
      const anchor = anchorLeaf(handler.path, new Set(leafPaths));
      const covered = anchor !== null || ROUTE_TABLE_GAPS.includes(label);
      expect(
        covered,
        `${label} matches no leaf in @shared API_ROUTES. Either the route is ` +
          'gone from the app (drop the handler) or shared/src/api/routes.ts is ' +
          'missing the constant (add it, or record it in ROUTE_TABLE_GAPS with ' +
          'a pointer to the backend route).',
      ).toBe(true);
    },
  );
});
