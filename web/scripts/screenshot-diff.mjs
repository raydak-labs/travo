// Compares two screenshot directories and writes, for every changed image, a side-by-side
// "Before | After" PNG with red boxes around the changed regions, plus index.md listing changed,
// new and removed shots. The pixel work runs in headless Chrome (canvas), so no npm dependency.
// Usage: node web/scripts/screenshot-diff.mjs --base DIR --head DIR --out DIR [--match REGEX]
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { launchChrome } from './cdp.mjs';

/** Parses `--key value` pairs; unknown keys are an error so a typo never compares everything. */
export function parseArgs(argv) {
  const out = { match: '.*' };
  for (let i = 0; i < argv.length; i += 2) {
    const key = argv[i]?.replace(/^--/, '');
    if (!['base', 'head', 'out', 'match'].includes(key ?? ''))
      throw new Error(`unknown flag ${argv[i]}`);
    out[key] = argv[i + 1];
  }
  for (const k of ['base', 'head', 'out']) if (!out[k]) throw new Error(`--${k} is required`);
  return out;
}

/** Splits the two listings into changed candidates (in both), new and removed names. */
export function pairUp(baseNames, headNames, match) {
  const re = new RegExp(match);
  const base = new Set(baseNames.filter((n) => n.endsWith('.png') && re.test(n)));
  const head = headNames.filter((n) => n.endsWith('.png') && re.test(n)).sort();
  return {
    both: head.filter((n) => base.has(n)),
    added: head.filter((n) => !base.has(n)),
    removed: [...base].filter((n) => !head.includes(n)).sort(),
  };
}

/**
 * Merges changed grid blocks into boxes: blocks closer than `gap` blocks join one box. Pure, so
 * the grouping is unit-tested; the browser only reports which blocks changed.
 */
export function boxes(blocks, gap = 3) {
  const out = [];
  for (const [x, y] of blocks) {
    const hit = out.find(
      (b) => x >= b.x0 - gap && x <= b.x1 + gap && y >= b.y0 - gap && y <= b.y1 + gap,
    );
    if (hit) {
      hit.x0 = Math.min(hit.x0, x);
      hit.y0 = Math.min(hit.y0, y);
      hit.x1 = Math.max(hit.x1, x);
      hit.y1 = Math.max(hit.y1, y);
    } else out.push({ x0: x, y0: y, x1: x, y1: y });
  }
  // A merge can make two boxes overlap: repeat until stable.
  for (let merged = true; merged; ) {
    merged = false;
    for (let i = 0; i < out.length && !merged; i++) {
      for (let j = i + 1; j < out.length && !merged; j++) {
        const a = out[i];
        const b = out[j];
        if (a.x0 - gap <= b.x1 && b.x0 - gap <= a.x1 && a.y0 - gap <= b.y1 && b.y0 - gap <= a.y1) {
          out[i] = {
            x0: Math.min(a.x0, b.x0),
            y0: Math.min(a.y0, b.y0),
            x1: Math.max(a.x1, b.x1),
            y1: Math.max(a.y1, b.y1),
          };
          out.splice(j, 1);
          merged = true;
        }
      }
    }
  }
  return out;
}

const BLOCK = 8;

// Runs in the page: decodes both PNGs, lists changed 8 px blocks.
const changedBlocksJs = (a, b) => `(async () => {
  const load = (src) => new Promise((ok, fail) => { const i = new Image(); i.onload = () => ok(i); i.onerror = fail; i.src = src; });
  const [ia, ib] = await Promise.all([load(${JSON.stringify(a)}), load(${JSON.stringify(b)})]);
  const w = Math.max(ia.width, ib.width), h = Math.max(ia.height, ib.height);
  const px = (img) => { const c = new OffscreenCanvas(w, h); const g = c.getContext('2d'); g.drawImage(img, 0, 0); return g.getImageData(0, 0, w, h).data; };
  const da = px(ia), db = px(ib);
  const blocks = [];
  for (let by = 0; by < Math.ceil(h / ${BLOCK}); by++) {
    for (let bx = 0; bx < Math.ceil(w / ${BLOCK}); bx++) {
      let hit = false;
      for (let y = by * ${BLOCK}; y < Math.min(h, (by + 1) * ${BLOCK}) && !hit; y++) {
        for (let x = bx * ${BLOCK}; x < Math.min(w, (bx + 1) * ${BLOCK}); x++) {
          const i = (y * w + x) * 4;
          if (Math.abs(da[i] - db[i]) + Math.abs(da[i + 1] - db[i + 1]) + Math.abs(da[i + 2] - db[i + 2]) > 48) { hit = true; break; }
        }
      }
      if (hit) blocks.push([bx, by]);
    }
  }
  return { w, h, blocks };
})()`;

// Runs in the page: draws Before | After with the boxes on both, at half scale.
const composeJs = (a, b, w, h, rects, labels) => `(async () => {
  const load = (src) => new Promise((ok) => { const i = new Image(); i.onload = () => ok(i); i.src = src; });
  const imgs = await Promise.all(${JSON.stringify([a, b].filter(Boolean))}.map(load));
  const s = 0.5, head = 56, pad = 24, cw = ${w} * s, ch = ${h} * s;
  const c = document.createElement('canvas');
  c.width = imgs.length * cw + (imgs.length + 1) * pad; c.height = ch + head + pad;
  const g = c.getContext('2d');
  g.fillStyle = '#ffffff'; g.fillRect(0, 0, c.width, c.height);
  imgs.forEach((img, k) => {
    const x = pad + k * (cw + pad);
    g.fillStyle = '#111827'; g.font = '600 28px system-ui, sans-serif';
    g.fillText(${JSON.stringify(labels)}[k], x, 38);
    g.drawImage(img, x, head, img.width * s, img.height * s);
    g.strokeStyle = '#d1d5db'; g.lineWidth = 1; g.strokeRect(x - 0.5, head - 0.5, cw + 1, ch + 1);
    g.strokeStyle = '#ef4444'; g.lineWidth = 4;
    for (const r of ${JSON.stringify(rects)}) g.strokeRect(x + r.x * s - 4, head + r.y * s - 4, r.w * s + 8, r.h * s + 8);
  });
  return c.toDataURL('image/png').split(',')[1];
})()`;

const dataUrl = (path) => `data:image/png;base64,${readFileSync(path).toString('base64')}`;

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const { both, added, removed } = pairUp(
    readdirSync(args.base),
    readdirSync(args.head),
    args.match,
  );
  if (existsSync(args.out)) rmSync(args.out, { recursive: true });
  mkdirSync(args.out, { recursive: true });
  const chrome = await launchChrome();
  const changed = [];
  try {
    await chrome.send('Page.navigate', { url: 'about:blank' });
    for (const name of both) {
      const a = dataUrl(join(args.base, name));
      const b = dataUrl(join(args.head, name));
      const { w, h, blocks } = await chrome.evaluate(changedBlocksJs(a, b));
      if (blocks.length === 0) continue;
      const rects = boxes(blocks).map((r) => ({
        x: r.x0 * BLOCK,
        y: r.y0 * BLOCK,
        w: (r.x1 - r.x0 + 1) * BLOCK,
        h: (r.y1 - r.y0 + 1) * BLOCK,
      }));
      const png = await chrome.evaluate(composeJs(a, b, w, h, rects, ['Before', 'After']));
      writeFileSync(join(args.out, name), Buffer.from(png, 'base64'));
      changed.push({ name, regions: rects.length });
    }
    for (const name of added) {
      const a = dataUrl(join(args.head, name));
      const { w, h } = await chrome.evaluate(changedBlocksJs(a, a));
      const png = await chrome.evaluate(composeJs(a, null, w, h, [], ['New']));
      writeFileSync(join(args.out, name), Buffer.from(png, 'base64'));
    }
  } finally {
    chrome.close();
  }
  const lines = [
    `# Screenshot changes`,
    '',
    `Changed: ${changed.length}, new: ${added.length}, removed: ${removed.length}, unchanged: ${both.length - changed.length}.`,
    '',
    ...changed.map(
      (c) => `- changed: ${c.name} (${c.regions} region${c.regions === 1 ? '' : 's'})`,
    ),
    ...added.map((n) => `- new: ${n}`),
    ...removed.map((n) => `- removed: ${n}`),
    '',
  ];
  writeFileSync(join(args.out, 'index.md'), lines.join('\n'));
  console.log(lines.slice(2, 3).join(''));
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
}
