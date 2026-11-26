import { mkdirSync } from 'node:fs';
import { launchChrome } from './cdp.mjs';

const base = `http://localhost:${process.env.WEB_PORT ?? 18192}`;
const out = `web/screenshots/${process.env.PHASE ?? 'after'}`;
mkdirSync(out, { recursive: true });
const chrome = await launchChrome();
const pause = () => new Promise((r) => setTimeout(r, 100));
async function waitFor(expression) {
  for (let i = 0; i < 50; i++) {
    if (await chrome.evaluate(expression)) return;
    await pause();
  }
  throw new Error(`timed out waiting for ${expression}`);
}
const click = async (text) => {
  const clicked = await chrome.evaluate(
    `(() => { const el = Array.from(document.querySelectorAll('button,a')).find((node) => node.textContent.trim() === ${JSON.stringify(text)}); if (!el) return false; el.click(); return true; })()`,
  );
  if (!clicked) throw new Error(`missing control: ${text}`);
};
try {
  await chrome.open(`${base}/#/login`, { width: 1280, height: 800 });
  await chrome.evaluate(
    `(() => {const e=document.querySelector('input');e.value='mock-password';e.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('form').requestSubmit()})()`,
  );
  await waitFor(`location.hash === '#/'`);
  for (const width of [390, 1280])
    for (const scheme of ['light', 'dark']) {
      await chrome.open(`${base}/#/`, { width, height: 844, scheme });
      await chrome.screenshot(`${out}/dashboard-${width}-${scheme}.png`);
      if (width === 390) {
        await chrome.evaluate(`document.querySelector('[aria-controls="sidebar"]').click()`);
        await waitFor(`!!document.querySelector('[data-slot="sheet-content"]')`);
        await chrome.screenshot(`${out}/drawer-${width}-${scheme}.png`);
        await chrome.send('Input.dispatchKeyEvent', {
          type: 'keyDown',
          key: 'Escape',
          code: 'Escape',
        });
        await waitFor(`!document.querySelector('[data-slot="sheet-content"]')`);
      }
      await click('Connect to WiFi');
      await waitFor(
        `!!Array.from(document.querySelectorAll('button')).find((el) => el.textContent.trim() === 'Scan')`,
      );
      await click('Scan');
      await waitFor(`!!document.querySelector('[aria-label="Join Neighbor2"]')`);
      await chrome.screenshot(`${out}/wifi-${width}-${scheme}.png`);
      const opened = await chrome.evaluate(
        `(() => { const el = document.querySelector('[aria-label="Join Neighbor2"]'); if (!el) return false; el.click(); return true; })()`,
      );
      if (!opened) throw new Error('missing control: Join Neighbor2');
      await waitFor(`!!document.querySelector('[data-slot="dialog-content"]')`);
      await chrome.screenshot(`${out}/join-${width}-${scheme}.png`);
      await chrome.send('Input.dispatchKeyEvent', {
        type: 'keyDown',
        key: 'Escape',
        code: 'Escape',
      });
      await waitFor(`!document.querySelector('[data-slot="dialog-content"]')`);
      await chrome.open(`${base}/#/system`, { width, height: 844, scheme });
      await click('Restart router');
      await waitFor(`document.body.innerText.includes('Restart the router?')`);
      await chrome.evaluate(
        `Array.from(document.querySelectorAll('button')).find(el=>el.textContent.trim()==='Restart')?.scrollIntoView({block:'center'})`,
      );
      await chrome.screenshot(`${out}/reboot-${width}-${scheme}.png`);
    }
} finally {
  chrome.close();
}
