import { describe, expect, it } from 'vitest';
import { STATUS_KINDS, statusClass } from './status';

describe('statusClass', () => {
  it('covers the full vocabulary', () => {
    expect([...STATUS_KINDS]).toEqual(['ok', 'warn', 'danger', 'info', 'neutral', 'stale']);
  });

  it('maps every kind to the base class plus its own token class', () => {
    for (const kind of STATUS_KINDS) {
      expect(statusClass(kind)).toBe(`status status-${kind}`);
    }
  });
});
