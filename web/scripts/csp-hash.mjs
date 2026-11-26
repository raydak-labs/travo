// CSP source for the one inline script in dist/index.html (the theme script).
import { createHash } from 'node:crypto';

const SCRIPT = /<script(\s[^>]*)?>([\s\S]*?)<\/script>/gi;
const HAS_SRC = /(^|\s)src\s*=/i;

/** Returns `'sha256-<base64>'` of the content of the single `<script>` without `src`; throws otherwise. */
export function inlineScriptHash(html) {
  const inline = [...html.matchAll(SCRIPT)].filter((m) => !HAS_SRC.test(m[1] ?? ''));
  if (inline.length !== 1) {
    throw new Error(`expected exactly one inline <script>, found ${inline.length}`);
  }
  return `sha256-${createHash('sha256').update(inline[0][2]).digest('base64')}`;
}
