// Public UI seam checks against the mock daemon. HTTP stubs use generated API
// shapes for guarded states; no device payloads or real router are fabricated.
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { launchChrome } from './cdp.mjs';

const base = `http://localhost:${process.env.WEB_PORT ?? 18192}`;
const out = 'web/screenshots/after';
mkdirSync(out, { recursive: true });
const chrome = await launchChrome();
const pause = (ms = 100) => new Promise((r) => setTimeout(r, ms));
async function wait(expression) {
  for (let i = 0; i < 100; i++) {
    if (await chrome.evaluate(expression)) return;
    await pause();
  }
  throw new Error(`UI did not reach: ${expression}`);
}
async function check(expression, message) {
  assert.ok(await chrome.evaluate(expression), message);
}
async function key(key, code = key, modifiers = 0) {
  const windowsVirtualKeyCode = { Tab: 9, Enter: 13, Escape: 27, ' ': 32 }[key];
  await chrome.send('Input.dispatchKeyEvent', {
    type: 'keyDown',
    key,
    code,
    modifiers,
    windowsVirtualKeyCode,
    text: key === 'Enter' ? '\r' : key === ' ' ? ' ' : undefined,
  });
  await chrome.send('Input.dispatchKeyEvent', {
    type: 'keyUp',
    key,
    code,
    modifiers,
    windowsVirtualKeyCode,
  });
  await pause();
}
async function click(selector) {
  await wait(`!!document.querySelector(${JSON.stringify(selector)})`);
  const point = await chrome.evaluate(
    `(() => {const el=document.querySelector(${JSON.stringify(selector)});el.scrollIntoView({block:'center'});const r=el.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2};})()`,
  );
  await chrome.send('Input.dispatchMouseEvent', {
    type: 'mousePressed',
    button: 'left',
    clickCount: 1,
    ...point,
  });
  await chrome.send('Input.dispatchMouseEvent', {
    type: 'mouseReleased',
    button: 'left',
    clickCount: 1,
    ...point,
  });
  await pause();
}
async function focusText(text) {
  await chrome.evaluate(
    `Array.from((document.querySelector('[role="dialog"],[role="alertdialog"]') ?? document).querySelectorAll('button,a')).find(e=>e.textContent.trim()===${JSON.stringify(text)}).focus()`,
  );
}
async function activate(text, activation = 'Enter') {
  await focusText(text);
  await key(activation);
}
async function fill(selector, value) {
  await chrome.evaluate(
    `(() => { const el=document.querySelector(${JSON.stringify(selector)});el.focus();el.select(); })()`,
  );
  await chrome.send('Input.insertText', { text: value });
}
async function noOverflow() {
  await check(
    `document.documentElement.scrollWidth <= innerWidth && [...document.querySelectorAll('main,[role="dialog"]')].every(e => e.scrollWidth <= e.clientWidth + 1)`,
    'no horizontal overflow',
  );
}
// On-demand responses model API states, not router captures. Record paths/methods
// only: submitted passwords, profiles and keys must never enter the test output.
function installHttpSeams() {
  const real = window.fetch;
  window.calls = [];
  window.txnState = 'pending';
  window.updateState = 'idle';
  window.fetch = async (input, init) => {
    const request = new Request(input, init);
    const path = new URL(request.url).pathname;
    const method = request.method;
    window.calls.push({ path, method });
    const transaction = () => ({
      id: '1234567890abcdef',
      configs: ['wireless'],
      started: '2026-11-13T22:47:16Z',
      summary: 'UI test',
      state: window.txnState,
      window_seconds: 60,
    });
    if (path === '/api/v1/wifi/join' || path === '/api/v1/vpn/tunnel') {
      window.txnState = 'pending';
      return Response.json(transaction());
    }
    if (path.startsWith('/api/v1/transactions/')) {
      if (path.endsWith('/confirm')) window.txnState = 'confirmed';
      if (path.endsWith('/rollback')) window.txnState = 'rolled_back';
      return Response.json(transaction());
    }
    if (path === '/api/v1/vpn/profiles' && method === 'GET')
      return Response.json({
        profiles: [{ id: 'test', name: 'Test profile' }],
        selected_id: 'test',
      });
    if (path === '/api/v1/system/update/check')
      return Response.json({
        status: 'available',
        running_version: '0.1.0',
        latest_version: '0.1.1',
        checked_at: null,
      });
    if (path === '/api/v1/system/update/stage')
      return Response.json({ status: 'verified', version: '0.1.1' });
    if (path === '/api/v1/system/update/apply') {
      if (method === 'POST') window.updateState = 'succeeded';
      return Response.json({ state: window.updateState, version: '0.1.1' });
    }
    if (path === '/api/v1/system/reboot')
      return Response.json({ error: 'Pending', code: 'transaction_pending' }, { status: 409 });
    return real(input, init);
  };
}
const writeCount = (path) =>
  `window.calls.filter(c=>c.path===${JSON.stringify(path)} && c.method==='POST').length`;
async function wifi(width, scheme) {
  await chrome.open(`${base}/#/`, { width, height: 844, scheme });
  await click('a[href="#/wifi"]:not(nav a)');
  await wait(
    `!!Array.from(document.querySelectorAll('button')).find(e=>e.textContent.trim()==='Scan')`,
  );
  await activate('Scan', ' ');
  await wait(`!!document.querySelector('[aria-label="Join Neighbor2"]')`);
  await click('[aria-label="Join Neighbor2"]');
  await wait(`!!document.querySelector('[role="dialog"] input')`);
  await check(
    `document.activeElement === document.querySelector('[role="dialog"] input')`,
    'join autofocus',
  );
  await check(`${writeCount('/api/v1/wifi/join')} === 0`, 'opening join does not write');
  await check(
    `document.querySelector('[role="dialog"] input').minLength===8 && document.querySelector('[role="dialog"] input').maxLength===63`,
    'password constraints',
  );
  await key('Tab');
  await check(`document.activeElement.textContent.trim()==='Cancel'`, 'join Tab order');
  await key('Tab');
  // Cancel, Save, Join, then back to the password field.
  await key('Tab');
  await key('Tab');
  await check(
    `document.activeElement === document.querySelector('[role="dialog"] input')`,
    'join focus trap',
  );
  await check(
    `getComputedStyle(document.activeElement).outlineStyle !== 'none'`,
    'visible keyboard focus',
  );
  await fill('[role="dialog"] input', 'short');
  await activate('Join');
  await check(`${writeCount('/api/v1/wifi/join')} === 0`, 'invalid key does not submit');
  await chrome.screenshot(`${out}/join-invalid-${width}-${scheme}.png`);
  await key('Escape');
  await wait(`!document.querySelector('[role="dialog"]')`);
  await check(
    `document.activeElement.getAttribute('aria-label')==='Join Neighbor2'`,
    'Escape restores join trigger',
  );
  await key('Enter');
  await wait(`!!document.querySelector('[role="dialog"] input')`);
  await check(
    `document.querySelector('[role="dialog"] input').value === ''`,
    'Escape cleared secret',
  );
  await fill('[role="dialog"] input', 'test-password');
  await activate('Cancel', ' ');
  await wait(`!document.querySelector('[role="dialog"]')`);
  await check(
    `document.activeElement.getAttribute('aria-label')==='Join Neighbor2'`,
    'Cancel restores trigger',
  );
  await key('Enter');
  await wait(`!!document.querySelector('[role="dialog"] input')`);
  await check(
    `document.querySelector('[role="dialog"] input').value === ''`,
    'Cancel cleared secret',
  );
  await fill('[role="dialog"] input', 'test-password');
  await key('Enter');
  await wait(
    `${writeCount('/api/v1/wifi/join')} === 1 && !document.querySelector('[role="dialog"]')`,
  );
  await wait(`document.body.textContent.includes('Keep this change?')`);
  await activate('Keep change');
  await wait(`document.body.textContent.includes('Change confirmed')`);
  await noOverflow();
  await chrome.evaluate('history.back()');
  await wait(`location.hash==='#/' && !!document.querySelector('a[href="#/wifi"]:not(nav a)')`);
  if (width === 390) {
    await click('[aria-controls="sidebar"]');
    await wait(`!!document.querySelector('[data-slot="sheet-content"]')`);
    await check(
      `document.getElementById(document.querySelector('[aria-controls="sidebar"]').getAttribute('aria-controls'))?.getAttribute('role') === 'dialog'`,
      'Menu controls the open drawer',
    );
    await check(`document.activeElement.hasAttribute('data-dialog-close')`, 'drawer focuses Close');
    await key('Tab');
    await check(
      `document.activeElement.getAttribute('href')==='#/'`,
      'drawer focus wraps to first nav link',
    );
    await key('Tab', 'Tab', 8);
    await check(
      `document.activeElement.hasAttribute('data-dialog-close')`,
      'drawer reverse Tab traps focus',
    );
    await chrome.screenshot(`${out}/drawer-focus-${width}-${scheme}.png`);
    await key('Escape');
    await wait(`!document.querySelector('[role="dialog"]')`);
    await check(
      `document.activeElement.getAttribute('aria-controls')==='sidebar'`,
      'drawer restores Menu',
    );
  }
  console.log(`keyboard/Join/Back/drawer ${width} ${scheme}: pass`);
}
async function systemAndVpn(width, scheme) {
  await chrome.open(`${base}/#/system`, { width, height: 844, scheme });
  await activate('Restart router', ' ');
  await wait(`!!document.querySelector('[role="alertdialog"]')`);
  await check(`${writeCount('/api/v1/system/reboot')} === 0`, 'reboot confirmation does not write');
  await chrome.screenshot(`${out}/reboot-confirm-${width}-${scheme}.png`);
  await activate('Cancel');
  await wait(`!document.querySelector('[role="alertdialog"]')`);
  await check(`${writeCount('/api/v1/system/reboot')} === 0`, 'reboot Cancel does not write');
  await activate('Restart router');
  await wait(`!!document.querySelector('[role="alertdialog"]')`);
  await activate('Restart router');
  await wait(`!document.querySelector('[role="alertdialog"]')`);
  await wait(
    `document.body.textContent.includes('Confirm or roll back the pending change first.')`,
  );
  await check(
    `${writeCount('/api/v1/system/reboot')} === 1`,
    'reboot submits only after confirmation',
  );
  await activate('Apply update', ' ');
  await wait(`!!document.querySelector('[role="alertdialog"]')`);
  await check(
    `${writeCount('/api/v1/system/update/apply')} === 0`,
    'update confirmation does not write',
  );
  await chrome.screenshot(`${out}/update-confirm-${width}-${scheme}.png`);
  await activate('Cancel');
  await wait(`!document.querySelector('[role="alertdialog"]')`);
  await check(`${writeCount('/api/v1/system/update/apply')} === 0`, 'update Cancel does not write');
  await activate('Apply update');
  await wait(`!!document.querySelector('[role="alertdialog"]')`);
  await activate('Apply update');
  await wait(`!document.querySelector('[role="alertdialog"]')`);
  await wait(`document.body.textContent.includes('Version 0.1.1 installed.')`);
  await check(`${writeCount('/api/v1/system/update/apply')} === 1`, 'update submits exactly once');
  await noOverflow();
  await chrome.open(`${base}/#/vpn`, { width, height: 844, scheme });
  await wait(`document.body.textContent.includes('Test profile')`);
  await check(
    `Array.from(document.querySelectorAll('button')).find(e=>e.textContent.trim()==='Import profile').disabled`,
    'empty VPN import disabled',
  );
  await check(
    `(([t]) => t.required && t.spellcheck === false)([...document.querySelectorAll('textarea')].slice(-1))`,
    'VPN textarea contract',
  );
  await activate('Enable tunnel');
  await wait(`document.body.textContent.includes('Keep this change?')`);
  await chrome.screenshot(`${out}/vpn-pending-${width}-${scheme}.png`);
  await activate('Roll back', ' ');
  await wait(`document.body.textContent.includes('Tunnel change rolled back.')`);
  await activate('Enable tunnel');
  await wait(`document.body.textContent.includes('Keep this change?')`);
  await activate('Keep change');
  await wait(`document.body.textContent.includes('Tunnel change confirmed.')`);
  await noOverflow();
  console.log(`reboot/update/VPN controls ${width} ${scheme}: pass`);
}

try {
  await chrome.open(`${base}/#/login`, { width: 1280, height: 800 });
  await fill('input', 'mock-password');
  await activate('Sign in');
  await wait(`location.hash==='#/' && !!document.querySelector('dl')`);
  await chrome.send('Page.addScriptToEvaluateOnNewDocument', {
    source: `(${installHttpSeams.toString()})()`,
  });
  for (const width of [390, 1280])
    for (const scheme of ['light', 'dark']) {
      await wifi(width, scheme);
      await systemAndVpn(width, scheme);
    }
} finally {
  chrome.close();
}
