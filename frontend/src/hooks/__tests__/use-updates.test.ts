import { describe, expect, it } from 'vitest';
import { updatesRefetchInterval } from '../use-updates';

describe('updatesRefetchInterval', () => {
  // The router's first check runs 2 minutes after it starts. A page opened
  // before that must not wait 10 more minutes to show what it found.
  it('polls every 30 s until the first check has run', () => {
    expect(updatesRefetchInterval(undefined)).toBe(30_000);
    expect(updatesRefetchInterval({ checked_at: '' })).toBe(30_000);
  });

  it('polls every 10 minutes once a check has run', () => {
    expect(updatesRefetchInterval({ checked_at: '2026-10-25T18:52:27Z' })).toBe(600_000);
  });
});
