import { describe, expect, it } from 'vitest';
import { attachable, boxes, indexLines, pairUp, parseArgs } from './screenshot-diff.mjs';
import { parseMarks, rectsPath, shotKey, toShotRects } from './screenshot-marks.mjs';

describe('screenshot diff', () => {
  it('needs base, head and out, and refuses unknown flags', () => {
    expect(parseArgs(['--base', 'a', '--head', 'b', '--out', 'c'])).toEqual({
      base: 'a',
      head: 'b',
      out: 'c',
      match: '.*',
    });
    expect(parseArgs(['--base', 'a', '--head', 'b', '--out', 'c', '--marks', 'm.json']).marks).toBe(
      'm.json',
    );
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

  it('keys a marks file by shot name without width and theme', () => {
    expect(shotKey('servicesadguard-390-light.png')).toBe('servicesadguard');
    expect(shotKey('/x/internet-connect-dialog-1280-dark.png')).toBe('internet-connect-dialog');
    expect(rectsPath('/x/dashboard-1280-light.png')).toBe('/x/dashboard-1280-light.marks.json');
  });

  it('accepts only shot -> selector lists', () => {
    expect(parseMarks('{"dashboard": ["main h2"]}')).toEqual({ dashboard: ['main h2'] });
    expect(() => parseMarks('["main"]')).toThrow('must be an object');
    expect(() => parseMarks('{"dashboard": "main"}')).toThrow('list of CSS selectors');
    expect(() => parseMarks('{"dashboard": [""]}')).toThrow('list of CSS selectors');
  });

  it('turns element rects into screenshot pixels clipped to the viewport', () => {
    const viewport = { width: 390, height: 844 };
    expect(
      toShotRects(
        [
          { x: 10, y: 20, width: 100, height: 30 },
          { x: 300, y: 830, width: 200, height: 40 },
          { x: 0, y: 900, width: 50, height: 50 },
          { x: 5, y: 5, width: 0, height: 10 },
        ],
        viewport,
        2,
      ),
    ).toEqual([
      { x: 20, y: 40, w: 200, h: 60 },
      { x: 600, y: 1660, w: 180, h: 28 },
    ]);
  });

  it('attaches every shot without marks and only marked shots with them', () => {
    expect(attachable('system-reboot-dialog-1280-light.png', undefined)).toBe(true);
    const marks = { dashboard: ['main'] };
    expect(attachable('dashboard-1280-light.png', marks)).toBe(true);
    expect(attachable('system-reboot-dialog-1280-light.png', marks)).toBe(false);
  });

  it('lists unattached changes in the index so they can be explained', () => {
    const lines = indexLines({
      changed: [
        { name: 'dashboard-1280-light.png', attached: true, regions: 1 },
        { name: 'system-reboot-dialog-1280-light.png', attached: false },
      ],
      added: [{ name: 'new-1280-light.png', attached: false }],
      removed: ['gone-1280-light.png'],
      unchanged: 3,
      marked: true,
    });
    expect(lines[2]).toBe(
      'Changed: 2, new: 1, removed: 1, unchanged: 3. Attached: 1 (marked shots only).',
    );
    expect(lines).toContain('- changed: dashboard-1280-light.png (1 region)');
    expect(lines).toContain('- changed, not attached: system-reboot-dialog-1280-light.png');
    expect(lines).toContain('- new, not attached: new-1280-light.png');
    expect(lines).toContain('- removed: gone-1280-light.png');
    expect(
      indexLines({ changed: [], added: [], removed: [], unchanged: 0, marked: false })[2],
    ).toBe('Changed: 0, new: 0, removed: 0, unchanged: 0.');
  });
});
