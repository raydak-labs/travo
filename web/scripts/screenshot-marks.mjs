// Marks file for screenshot diffs (docs/testing.md "Screenshots"): which shots a PR means to
// change and which elements to box in them. Shared by screenshots.mjs (records the rects at
// capture time) and screenshot-diff.mjs (attaches only marked shots and draws those rects).
import { readFileSync } from 'node:fs';
import { basename } from 'node:path';

/** `servicesadguard-390-light.png` -> `servicesadguard`: the key a marks file uses. */
export function shotKey(fileName) {
  return basename(fileName).replace(/-\d+-(light|dark)\.png$/, '');
}

/** The `.marks.json` file stored next to a shot. */
export function rectsPath(pngPath) {
  return pngPath.replace(/\.png$/, '.marks.json');
}

/** Validates `{ "<shot>": ["<selector>", ...] }`; a malformed file is an error, not "no marks". */
export function parseMarks(text) {
  const marks = JSON.parse(text);
  if (marks === null || typeof marks !== 'object' || Array.isArray(marks))
    throw new Error('marks file must be an object of shot name -> selector list');
  for (const [shot, selectors] of Object.entries(marks)) {
    if (!Array.isArray(selectors) || !selectors.every((s) => typeof s === 'string' && s !== ''))
      throw new Error(`marks for ${shot} must be a list of CSS selectors`);
  }
  return marks;
}

export function readMarks(path) {
  return path ? parseMarks(readFileSync(path, 'utf8')) : undefined;
}

/**
 * Turns element rects (CSS px, viewport-relative) into screenshot pixels, clipped to the
 * viewport; elements outside it or without size are dropped.
 */
export function toShotRects(rects, viewport, scale) {
  const out = [];
  for (const r of rects) {
    const x0 = Math.max(0, r.x);
    const y0 = Math.max(0, r.y);
    const x1 = Math.min(viewport.width, r.x + r.width);
    const y1 = Math.min(viewport.height, r.y + r.height);
    if (x1 <= x0 || y1 <= y0) continue;
    out.push({
      x: Math.round(x0 * scale),
      y: Math.round(y0 * scale),
      w: Math.round((x1 - x0) * scale),
      h: Math.round((y1 - y0) * scale),
    });
  }
  return out;
}

/** Page expression: every match of each selector, as CSS-pixel rects plus viewport and scale. */
export function measureJs(selectors) {
  return `(() => ({
    rects: ${JSON.stringify(selectors)}.flatMap((s) => [...document.querySelectorAll(s)])
      .map((el) => el.getBoundingClientRect())
      .map((r) => ({ x: r.x, y: r.y, width: r.width, height: r.height })),
    viewport: { width: innerWidth, height: innerHeight },
    scale: devicePixelRatio,
  }))()`;
}
