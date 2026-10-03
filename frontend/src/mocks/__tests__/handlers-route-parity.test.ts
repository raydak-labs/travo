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

/**
 * MSW stores the resolved path on each handler, so the parity check compares
 * real values instead of grepping source text. RegExp handlers (used for the
 * wifi AP PUT wildcard) stringify; treat any string containing the leaf path as
 * a match.
 */
function handledPaths(): string[] {
  return (handlers as unknown as { info?: { path?: unknown } }[]).map((handler) =>
    String(handler.info?.path),
  );
}

describe('mock handler route parity', () => {
  const leaves = collectLeaves(API_ROUTES as unknown as RouteTree);
  const paths = handledPaths();

  it('walks the whole route table', () => {
    // Guards the walk itself: a parse regression would make the check vacuous.
    expect(leaves.size).toBeGreaterThan(100);
  });

  it.each([...leaves.entries()])('has a handler for %s (%s)', (_name, path) => {
    const covered = paths.some((handled) => handled.includes(path));
    expect(
      covered,
      `No MSW handler covers ${path}. Add one to src/mocks/handlers.ts: an unhandled route is ` +
        'bypassed to the network in dev and any test asserting against it is non-deterministic.',
    ).toBe(true);
  });
});
