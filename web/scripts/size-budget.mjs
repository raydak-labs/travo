import { readdirSync, readFileSync } from 'node:fs';
import { join, relative, resolve, sep } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const KB = 1024;
const SHELL_LIMIT = 128 * KB;
const CHUNK_LIMIT = 16 * KB;
const TOTAL_LIMIT = 256 * KB;
const TOTAL_ASSET = /\.(html|js|css|svg)$/;

function resolveAsset(reference, importer = 'index.html') {
  if (/^(?:[a-z][a-z\d+.-]*:|\/\/|#)/i.test(reference)) return undefined;
  const url = new URL(reference, `https://size-budget.invalid/${importer}`);
  return decodeURIComponent(url.pathname.slice(1));
}

function attribute(tag, name) {
  const match = tag.match(new RegExp(`\\s${name}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s>]+))`, 'i'));
  return match ? (match[1] ?? match[2] ?? match[3]) : undefined;
}

function addHtmlEntries(html, add) {
  for (const match of html.matchAll(/<(script|link)\b[^>]*>/gi)) {
    const reference = attribute(match[0], match[1].toLowerCase() === 'script' ? 'src' : 'href');
    if (reference === undefined) continue;
    const name = resolveAsset(reference);
    if (name !== undefined) add(name);
  }
}

function addStaticImports(source, importer, add) {
  const staticImport = /(?:\bimport\s*|\bfrom\s*)["']([^"']+)["']/g;
  for (const match of source.matchAll(staticImport)) {
    const name = resolveAsset(match[1], importer);
    if (name?.endsWith('.js')) add(name);
  }
}

/** Check gzip byte sizes without reading the filesystem. Names are relative to dist/. */
export function checkBudget({ files, html, sources }) {
  const shellFiles = new Set(['index.html']);
  const pendingScripts = [];
  const addShellFile = (name) => {
    if (shellFiles.has(name)) return;
    shellFiles.add(name);
    if (name.endsWith('.js')) pendingScripts.push(name);
  };

  addHtmlEntries(html, addShellFile);
  while (pendingScripts.length > 0) {
    const name = pendingScripts.shift();
    addStaticImports(sources.get(name) ?? '', name, addShellFile);
  }

  const shell = [...shellFiles].reduce((sum, name) => sum + (files.get(name) ?? 0), 0);
  const total = [...files].reduce(
    (sum, [name, bytes]) => sum + (TOTAL_ASSET.test(name) ? bytes : 0),
    0,
  );
  const chunks = [...files]
    .filter(([name]) => name.endsWith('.js') && !shellFiles.has(name))
    .map(([name, bytes]) => ({ name, bytes }))
    .sort((a, b) => a.name.localeCompare(b.name));

  const violations = [];
  if (shell > SHELL_LIMIT) {
    violations.push(`shell ${shell} B exceeds ${SHELL_LIMIT} B (128 KB gzip)`);
  }
  for (const { name, bytes } of chunks) {
    if (bytes > CHUNK_LIMIT) {
      violations.push(`chunk ${name} ${bytes} B exceeds ${CHUNK_LIMIT} B (16 KB gzip)`);
    }
  }
  if (total > TOTAL_LIMIT) {
    violations.push(`total ${total} B exceeds ${TOTAL_LIMIT} B (256 KB gzip)`);
  }

  return { shell, total, chunks, violations };
}

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(path);
    else if (entry.name.endsWith('.gz')) yield path;
  }
}

function readDist(dist) {
  const files = new Map();
  const sources = new Map();
  for (const path of walk(dist)) {
    const name = relative(dist, path.slice(0, -3)).split(sep).join('/');
    files.set(name, readFileSync(path).length);
    if (name.endsWith('.js')) sources.set(name, readFileSync(join(dist, name), 'utf8'));
  }
  return { files, html: readFileSync(join(dist, 'index.html'), 'utf8'), sources };
}

function formatKB(bytes) {
  return `${(bytes / KB).toFixed(2)} KB`;
}

function main() {
  const dist = fileURLToPath(new URL('../dist/', import.meta.url));
  const result = checkBudget(readDist(dist));
  console.log(`shell ${formatKB(result.shell)} / 128 KB gzip (${result.shell} B)`);
  console.log(`total ${formatKB(result.total)} / 256 KB gzip (${result.total} B)`);
  for (const { name, bytes } of result.chunks) {
    console.log(`chunk ${name} ${formatKB(bytes)} / 16 KB gzip (${bytes} B)`);
  }
  for (const violation of result.violations) console.error(`size budget: ${violation}`);
  if (result.violations.length > 0) process.exitCode = 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) main();
