import { describe, expect, it } from 'vitest';
import { checkBudget } from './size-budget.mjs';

describe('checkBudget', () => {
  it('includes transitive static imports and HTML assets, but not dynamic imports', () => {
    const result = checkBudget({
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
      violations: [],
    });
  });

  it('reports an over-limit non-shell chunk', () => {
    const result = checkBudget({
      files: new Map([
        ['index.html', 1],
        ['index.js', 1],
        ['lazy.js', 24 * 1024 + 1],
      ]),
      html: '<script src="/index.js"></script>',
      sources: new Map([['index.js', 'import("./lazy.js");']]),
    });

    expect(result.chunks).toEqual([{ name: 'lazy.js', bytes: 24577 }]);
    expect(result.violations).toHaveLength(1);
    expect(result.violations[0]).toMatch(/chunk lazy\.js.*24 KB/);
  });

  it('reports an over-limit shell without also counting its entry as a chunk', () => {
    const result = checkBudget({
      files: new Map([
        ['index.html', 1],
        ['index.js', 128 * 1024],
      ]),
      html: '<script src="/index.js"></script>',
      sources: new Map([['index.js', '']]),
    });

    expect(result.shell).toBe(128 * 1024 + 1);
    expect(result.chunks).toEqual([]);
    expect(result.violations).toHaveLength(1);
    expect(result.violations[0]).toMatch(/shell.*128 KB/);
  });

  it('reports an over-limit total independently of the shell and chunk limits', () => {
    const result = checkBudget({
      files: new Map([
        ['index.html', 1],
        ['index.js', 1],
        ['extra.css', 320 * 1024 - 1],
      ]),
      html: '<script src="/index.js"></script>',
      sources: new Map([['index.js', '']]),
    });

    expect(result.shell).toBe(2);
    expect(result.total).toBe(320 * 1024 + 1);
    expect(result.violations).toHaveLength(1);
    expect(result.violations[0]).toMatch(/total.*320 KB/);
  });
});
