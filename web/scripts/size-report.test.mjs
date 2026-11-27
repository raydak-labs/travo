import { describe, expect, it } from 'vitest';
import { reportSizes } from './size-report.mjs';

describe('reportSizes', () => {
  it('includes transitive static imports and HTML assets, but not dynamic imports', () => {
    const result = reportSizes({
      files: new Map([
        ['index.html', 10],
        ['index.js', 20],
        ['shared.js', 30],
        ['transitive.js', 40],
        ['lazy.js', 50],
        ['style.css', 60],
        ['logo.svg', 70],
        ['preload.js', 80],
        ['preloaded-dep.js', 90],
      ]),
      html: `
        <script type="module" src="/index.js"></script>
        <link rel="stylesheet" href="/style.css">
        <link rel="icon" href="/logo.svg">
        <link rel="modulepreload" href="./preload.js">
      `,
      sources: new Map([
        ['index.js', 'import"./shared.js"; import("./lazy.js");'],
        ['shared.js', 'export { value } from "./transitive.js";'],
        ['preload.js', 'import "./preloaded-dep.js";'],
      ]),
    });

    expect(result).toEqual({
      shell: 400,
      total: 450,
      chunks: [{ name: 'lazy.js', bytes: 50 }],
    });
  });

  it('lists non-shell JS chunks sorted by name without counting shell entries as chunks', () => {
    const result = reportSizes({
      files: new Map([
        ['index.html', 1],
        ['index.js', 200 * 1024],
        ['b.js', 30 * 1024],
        ['a.js', 40 * 1024],
        ['license.txt', 500],
      ]),
      html: '<script src="/index.js"></script>',
      sources: new Map([['index.js', 'import("./a.js"); import("./b.js");']]),
    });

    expect(result).toEqual({
      shell: 200 * 1024 + 1,
      total: 270 * 1024 + 1,
      chunks: [
        { name: 'a.js', bytes: 40 * 1024 },
        { name: 'b.js', bytes: 30 * 1024 },
      ],
    });
  });
});
