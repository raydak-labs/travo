// Minimal headless-Chrome driver over the DevTools protocol. Shared by browser checks and
// screenshots. Node built-ins only (child_process, fetch, WebSocket): no npm dependency.
import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, join } from 'node:path';

// CDP_PORT lets parallel browser checks (one per worktree) run side by side.
const DEBUG_PORT = Number(process.env.CDP_PORT ?? 9333);
const MAC_CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Picks the Chrome binary: $CHROME, then the macOS app, then the first common name on PATH. */
export function findChrome(env = process.env, exists = existsSync) {
  if (env.CHROME) {
    if (exists(env.CHROME)) return env.CHROME;
    throw new Error(`Chrome not found: set CHROME (${env.CHROME} does not exist)`);
  }
  if (exists(MAC_CHROME)) return MAC_CHROME;
  for (const name of ['google-chrome', 'chromium', 'chromium-browser']) {
    for (const dir of (env.PATH ?? '').split(delimiter)) {
      const p = join(dir, name);
      if (dir && exists(p)) return p;
    }
  }
  throw new Error('Chrome not found: set CHROME');
}

async function waitForPageTarget(timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    try {
      const res = await fetch(`http://127.0.0.1:${DEBUG_PORT}/json`);
      const page = (await res.json()).find((t) => t.type === 'page');
      if (page) return page;
    } catch {
      // Chrome has not opened the debugging port yet
    }
    if (Date.now() > deadline) throw new Error('Chrome did not expose a page target in 10 s');
    await sleep(100);
  }
}

/** Starts headless Chrome and connects to its first page. Call close() when done. */
export async function launchChrome({ userDataDir } = {}) {
  const binary = findChrome();
  // Another browser on the debug port would be driven silently instead of ours.
  const taken = await fetch(`http://127.0.0.1:${DEBUG_PORT}/json/version`).then(
    () => true,
    () => false,
  );
  if (taken) throw new Error(`port ${DEBUG_PORT} is in use: stop the other Chrome first`);
  const ownDir = userDataDir === undefined;
  const dir = userDataDir ?? mkdtempSync(join(tmpdir(), 'cdp-'));
  const chrome = spawn(
    binary,
    [
      '--headless=new',
      '--disable-gpu',
      '--no-first-run',
      `--remote-debugging-port=${DEBUG_PORT}`,
      `--user-data-dir=${dir}`,
      'about:blank',
    ],
    { stdio: 'ignore' },
  );
  const spawnFailed = new Promise((_, reject) =>
    chrome.once('error', (err) => reject(new Error(`Chrome failed to start: ${err.message}`))),
  );

  let ws;
  let nextId = 1;
  const pending = new Map();

  const close = () => {
    try {
      ws?.close();
    } catch {
      // socket already closed
    }
    const removeDir = () => ownDir && rmSync(dir, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    if (chrome.exitCode === null && chrome.signalCode === null) {
      chrome.once('exit', removeDir);
      chrome.kill('SIGTERM');
    } else removeDir();
  };

  try {
    const page = await Promise.race([waitForPageTarget(10_000), spawnFailed]);
    ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => {
      ws.addEventListener('open', resolve, { once: true });
      ws.addEventListener('error', () => reject(new Error('CDP websocket failed to connect')), {
        once: true,
      });
    });
  } catch (err) {
    close();
    throw err;
  }

  ws.addEventListener('message', (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id === undefined) return; // protocol events are not used here
    const waiter = pending.get(msg.id);
    if (!waiter) return;
    pending.delete(msg.id);
    if (msg.error) waiter.reject(new Error(`${msg.error.message}: ${msg.error.data ?? ''}`));
    else waiter.resolve(msg.result);
  });

  function send(method, params = {}) {
    const id = nextId++;
    return new Promise((resolve, reject) => {
      pending.set(id, { resolve, reject });
      ws.send(JSON.stringify({ id, method, params }));
    });
  }

  async function evaluate(expression) {
    const result = await send('Runtime.evaluate', {
      expression,
      returnByValue: true,
      awaitPromise: true,
    });
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.text);
    return result.result.value;
  }

  let themeScript;
  /** Sets viewport and colour scheme, navigates, and waits until the page has settled. */
  async function open(url, { width, height, mobile = false, scheme = 'light' }) {
    await send('Page.enable');
    await send('Runtime.enable');
    await send('Emulation.setDeviceMetricsOverride', {
      width,
      height,
      deviceScaleFactor: 2,
      mobile,
    });
    await send('Emulation.setEmulatedMedia', {
      features: [{ name: 'prefers-color-scheme', value: scheme }],
    });
    // App ignores the OS scheme. Inject the requested theme before the boot script.
    if (themeScript)
      await send('Page.removeScriptToEvaluateOnNewDocument', { identifier: themeScript });
    ({ identifier: themeScript } = await send('Page.addScriptToEvaluateOnNewDocument', {
      source: `try{localStorage.setItem('theme',${JSON.stringify(scheme)})}catch(e){}`,
    }));
    // A hash-only change does not reload, and the theme is applied at boot: leave the page first.
    await send('Page.navigate', { url: 'about:blank' });
    await send('Page.navigate', { url });

    const deadline = Date.now() + 10_000;
    for (;;) {
      const settled = await evaluate(
        `location.href === ${JSON.stringify(url)} && document.readyState === 'complete' && !document.querySelector('[aria-busy="true"]')`,
      );
      if (settled) return;
      if (Date.now() > deadline) throw new Error(`${url} did not settle within 10 s`);
      await sleep(100);
    }
  }

  async function screenshot(path) {
    const { data } = await send('Page.captureScreenshot', { format: 'png' });
    writeFileSync(path, Buffer.from(data, 'base64'));
  }

  return { send, evaluate, open, screenshot, close };
}
