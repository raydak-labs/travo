import { describe, expect, it } from 'vitest';
import { boxes, pairUp, parseArgs } from './screenshot-diff.mjs';

describe('screenshot diff', () => {
  it('needs base, head and out, and refuses unknown flags', () => {
    expect(parseArgs(['--base', 'a', '--head', 'b', '--out', 'c'])).toEqual({
      base: 'a',
      head: 'b',
      out: 'c',
      match: '.*',
    });
    expect(() => parseArgs(['--base', 'a'])).toThrow('--head is required');
    expect(() => parseArgs(['--bsae', 'a'])).toThrow('unknown flag');
  });

  it('pairs shots into compared, new and removed', () => {
    const r = pairUp(['a.png', 'gone.png', 'x.txt'], ['a.png', 'new.png'], '.*');
    expect(r).toEqual({ both: ['a.png'], added: ['new.png'], removed: ['gone.png'] });
    expect(
      pairUp(
        ['a-1280-light.png', 'a-390-light.png'],
        ['a-1280-light.png', 'a-390-light.png'],
        '-1280-',
      ).both,
    ).toEqual(['a-1280-light.png']);
  });

  it('merges nearby changed blocks into one box and keeps distant ones apart', () => {
    expect(
      boxes([
        [0, 0],
        [1, 0],
        [2, 1],
      ]),
    ).toEqual([{ x0: 0, y0: 0, x1: 2, y1: 1 }]);
    expect(
      boxes([
        [0, 0],
        [50, 50],
      ]),
    ).toHaveLength(2);
    // Two boxes that grow into each other end up as one.
    expect(
      boxes(
        [
          [0, 0],
          [6, 0],
          [3, 0],
        ],
        3,
      ),
    ).toEqual([{ x0: 0, y0: 0, x1: 6, y1: 0 }]);
  });
});
