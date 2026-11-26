// Loads the UI from the daemon (which sets the CSP header) and fails on any script exception
// or Content Security Policy violation. Run via `mise run e2e:csp`.
import { launchChrome } from './cdp.mjs';

const DEBUG_PORT = 9333; // cdp.mjs's fixed port; a second client on the page target sees the events
// Anonymous visitors are redirected from / to #/login, and open() waits for the exact URL.
const BASE_URL =
  process.env.E2E_URL ?? `http://127.0.0.1:${process.env.DAEMON_PORT ?? 18081}/#/login`;

/** Returns a problem description when a CDP event is an exception or a CSP violation. */
export function problemOf(msg) {
  if (msg.method === 'Runtime.exceptionThrown') {
    const d = msg.params.exceptionDetails;
    return `exception: ${d.exception?.description ?? d.text}`;
  }
  if (
    msg.method === 'Log.entryAdded' &&
    msg.params.entry.text.includes('Content Security Policy')
  ) {
    return `CSP violation: ${msg.params.entry.text}`;
  }
  return undefined;
}

async function watch(problems) {
  const page = (await (await fetch(`http://127.0.0.1:${DEBUG_PORT}/json`)).json()).find(
    (t) => t.type === 'page',
  );
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve, { once: true });
    ws.addEventListener('error', () => reject(new Error('CDP watcher failed to connect')), {
      once: true,
    });
  });
  ws.addEventListener('message', (ev) => {
    const p = problemOf(JSON.parse(ev.data));
    if (p) problems.push(p);
  });
  ws.send(JSON.stringify({ id: 1, method: 'Log.enable' }));
  ws.send(JSON.stringify({ id: 2, method: 'Runtime.enable' }));
  return ws;
}

async function main() {
  const problems = [];
  const chrome = await launchChrome();
  try {
    const ws = await watch(problems);
    await chrome.open(BASE_URL, { width: 1280, height: 800 });
    await new Promise((r) => setTimeout(r, 500)); // late console entries
    ws.close();
    // The theme script is inline: it ran only if its CSP hash matched.
    const theme = await chrome.evaluate(`document.documentElement.getAttribute('data-theme')`);
    if (theme !== 'light' && theme !== 'dark')
      problems.push(`inline theme script did not run (${theme})`);
  } finally {
    chrome.close();
  }
  if (problems.length > 0) {
    for (const p of problems) console.error(p);
    process.exitCode = 1;
    return;
  }
  console.log(`no CSP violations or exceptions on ${BASE_URL}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    console.error(err.message);
    process.exitCode = 1;
  });
}
