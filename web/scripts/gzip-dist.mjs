// Writes dist/csp.json (hash of the inline script) and <file>.gz next to every text asset in dist/
// so the device server can serve them precompressed. csp.json itself is never gzipped.
import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { constants, gzipSync } from 'node:zlib';
import { inlineScriptHash } from './csp-hash.mjs';

const dist = new URL('../dist', import.meta.url).pathname;
const exts = /\.(html|js|css|svg|json|txt|map)$/;
const CSP_FILE = 'csp.json';

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) yield* walk(p);
    else if (exts.test(name) && name !== CSP_FILE) yield p;
  }
}

const html = readFileSync(join(dist, 'index.html'), 'utf8');
writeFileSync(
  join(dist, CSP_FILE),
  `${JSON.stringify({ scriptHashes: [inlineScriptHash(html)] })}\n`,
);

let total = 0;
for (const file of walk(dist)) {
  const gz = gzipSync(readFileSync(file), { level: constants.Z_BEST_COMPRESSION });
  writeFileSync(`${file}.gz`, gz);
  total += gz.length;
  console.log(`${file.slice(dist.length + 1)}.gz ${gz.length} B`);
}
console.log(`gzip total ${total} B`);
