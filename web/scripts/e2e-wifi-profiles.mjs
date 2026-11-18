// Saved uplinks (#72) against the mock daemon at 390 px in light and dark.
// Save, reorder and forget use the real mock store. The mock has no live wireless
// UCI, so only Select's transaction reply is stubbed with the generated TxnBody shape.
// Run: pnpm -C web build && scripts/with-mock.sh -- node web/scripts/e2e-wifi-profiles.mjs
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { launchChrome } from './cdp.mjs';

const base = `http://localhost:${process.env.WEB_PORT ?? 18092}`;
const out = 'web/screenshots/wifi-profiles';
const viewport = { width: 390, height: 844, mobile: true };
mkdirSync(out, { recursive: true });
const chrome = await launchChrome();
const pause = (ms = 100) => new Promise((r) => setTimeout(r, ms));

async function wait(expression) {
  for (let i = 0; i < 100; i++) {
    if (await chrome.evaluate(expression)) return;
    await pause();
  }
  const text = await chrome.evaluate(`document.body.innerText.slice(0, 800)`);
  throw new Error(`UI did not reach: ${expression}\n--- page text ---\n${text}`);
}
async function check(expression, message) {
  assert.ok(await chrome.evaluate(expression), message);
}
async function click(selector) {
  await wait(`!!document.querySelector(${JSON.stringify(selector)})`);
  await chrome.evaluate(`document.querySelector(${JSON.stringify(selector)}).click()`);
  await pause();
}
async function clickText(text, scope = 'document') {
  const find = `Array.from(${scope}.querySelectorAll('button')).find(e=>e.textContent.trim()===${JSON.stringify(text)})`;
  await wait(`!!${find}`);
  await chrome.evaluate(`${find}.click()`);
  await pause();
}
async function fill(selector, value) {
  await chrome.evaluate(
    `(() => { const el=document.querySelector(${JSON.stringify(selector)});el.focus();el.select(); })()`,
  );
  await chrome.send('Input.insertText', { text: value });
}
const savedOrder = `Array.from(document.querySelectorAll('ol[aria-label="Saved uplinks"] > li span.break-all')).map(e=>e.textContent.trim()).join(',')`;
const writes = (suffix) =>
  `window.calls.filter(c=>c.method!=='GET' && c.path.endsWith(${JSON.stringify(suffix)})).length`;

// Paths and methods only: keys must never enter the test output.
function installSeams() {
  const real = window.fetch;
  window.calls = [];
  window.fetch = async (input, init) => {
    const request = new Request(input, init);
    const path = new URL(request.url).pathname;
    window.calls.push({ path, method: request.method });
    const state = path.endsWith('/rollback') ? 'rolled_back' : 'pending';
    if (path.endsWith('/select') || path.startsWith('/api/v1/transactions/'))
      return Response.json(
        {
          id: '1234567890abcdef',
          configs: ['wireless', 'network', 'firewall'],
          started: '2026-11-16T10:00:00Z',
          summary: 'UI test',
          state,
          window_seconds: 60,
        },
        { status: path.endsWith('/select') ? 202 : 200 },
      );
    // Copying a Request consumes its body, so forward the copy.
    const response = await real(request);
    // The mock has no association; mark one saved SSID connected to check the forget warning.
    const connected = sessionStorage.getItem('connected');
    if (path === '/api/v1/wifi/profiles' && request.method === 'GET' && connected && response.ok) {
      const list = await response.json();
      list.connected_id = list.profiles.find((p) => p.ssid === connected)?.id ?? null;
      return Response.json(list);
    }
    return response;
  };
}

async function openWifi(scheme) {
  await chrome.open(`${base}/#/wifi`, { ...viewport, scheme });
  await wait(`document.body.textContent.includes('Saved uplinks')`);
}
async function shoot(name) {
  for (const scheme of ['light', 'dark']) {
    await openWifi(scheme);
    await wait(`${savedOrder} !== '' || document.body.textContent.includes('No saved uplinks')`);
    await check(`document.documentElement.scrollWidth <= innerWidth`, 'no horizontal overflow');
    await chrome.screenshot(`${out}/${name}-390-${scheme}.png`);
  }
}
async function save(ssid, key) {
  if (!(await chrome.evaluate(`!!document.querySelector('[aria-label="Join ${ssid}"]')`)))
    await clickText('Scan');
  await click(`[aria-label="Join ${ssid}"]`);
  await wait(`!!document.querySelector('[role="dialog"] input')`);
  await fill('[role="dialog"] input', key);
  await clickText('Save', `document.querySelector('[role="dialog"]')`);
}

try {
  await chrome.open(`${base}/#/login`, { width: 1280, height: 800 });
  await fill('input', 'mock-password');
  await clickText('Sign in');
  await wait(`location.hash==='#/' && !!document.querySelector('dl')`);
  await chrome.send('Page.addScriptToEvaluateOnNewDocument', {
    source: `(${installSeams.toString()})()`,
  });

  await openWifi('light');
  await check(`document.body.textContent.includes('No saved uplinks')`, 'empty list shown');
  await save('HomeUplink', 'test-password-1');
  await wait(`!document.querySelector('[role="dialog"]') && ${savedOrder} === 'HomeUplink'`);
  await save('Neighbor2', 'test-password-2');
  await wait(
    `!document.querySelector('[role="dialog"]') && ${savedOrder} === 'HomeUplink,Neighbor2'`,
  );
  await check(`${writes('/wifi/join')} === 0`, 'save never joins');
  await check(
    `!document.querySelector('ol[aria-label="Saved uplinks"] .status-ok')`,
    'mock has no association',
  );

  // A failed save keeps the entered key in the open dialog only.
  await save('HomeUplink', 'test-password-3');
  await wait(`document.body.textContent.includes('This uplink is already saved.')`);
  await check(
    `document.querySelector('[role="dialog"] input').value !== ''`,
    'key kept on failure',
  );
  await clickText('Cancel', `document.querySelector('[role="dialog"]')`);
  await wait(`!document.querySelector('[role="dialog"]')`);
  await shoot('saved-two');

  await openWifi('light');
  await check(
    `document.querySelector('[aria-label="Move HomeUplink up"]').disabled`,
    'top end disabled',
  );
  await check(
    `document.querySelector('[aria-label="Move Neighbor2 down"]').disabled`,
    'bottom end disabled',
  );
  await click('[aria-label="Move Neighbor2 up"]');
  await wait(`${savedOrder} === 'Neighbor2,HomeUplink'`);
  await shoot('reordered');

  await openWifi('light');
  await click('[aria-label="Select HomeUplink"]');
  await wait(`document.body.textContent.includes('Keep this change?')`);
  await clickText('Roll back');
  await wait(`document.body.textContent.includes('Rolled back')`);
  await check(`${savedOrder} === 'Neighbor2,HomeUplink'`, 'select keeps order');
  await chrome.screenshot(`${out}/select-rolled-back-390-light.png`);

  await chrome.evaluate(`sessionStorage.setItem('connected', 'Neighbor2')`);
  await openWifi('light');
  await wait(`!!document.querySelector('ol[aria-label="Saved uplinks"] .status-ok')`);
  await click('[aria-label="Forget Neighbor2"]');
  await wait(`document.body.textContent.includes('keeps the current connection up')`);
  await chrome.screenshot(`${out}/forget-connected-390-light.png`);
  await clickText(
    'Forget',
    `(document.querySelector('[role="alertdialog"]') ?? document.createElement('div'))`,
  );
  await wait(`${savedOrder} === 'HomeUplink'`);
  await shoot('forgot-one');
  console.log('saved uplinks 390 light/dark: pass');
} finally {
  chrome.close();
}
