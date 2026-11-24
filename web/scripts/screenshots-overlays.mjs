// Screenshots of the shared overlays (docs/web-ui.md): a confirmation dialog, the pending
// transaction toast and a result toast. Only the responses that start a flow are stubbed;
// nothing writes to the mock daemon.

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const PENDING_TXN = {
  configs: ['wireless'],
  id: 'screenshot-txn',
  started: '2026-11-13T22:47:16Z',
  state: 'pending',
  summary: 'wifi: disable TravelAP',
  window_seconds: 60,
};

/** Replaces the responses of matching requests: [method, path regexp, status, body]. */
function stubFetch(routes) {
  return `(() => {
    const routes = ${JSON.stringify(routes.map(([method, path, status, body]) => [method, path.source, status, body]))};
    const real = window.fetch;
    window.fetch = (input, init) => {
      const request = new Request(input, init);
      const path = new URL(request.url).pathname;
      const hit = routes.find(([method, re]) => method === request.method && new RegExp(re).test(path));
      if (hit) return Promise.resolve(Response.json(hit[3], { status: hit[2] }));
      return real(input, init);
    };
  })()`;
}

async function waitFor(chrome, expression, what) {
  for (let i = 0; i < 100; i++) {
    if (await chrome.evaluate(expression)) return;
    await sleep(100);
  }
  throw new Error(`timed out waiting for ${what}`);
}

const clickButton = (chrome, scope, text) =>
  chrome.evaluate(`(() => {
    const button = [...document.querySelectorAll(${JSON.stringify(`${scope} button`)})]
      .find((b) => b.textContent.trim() === ${JSON.stringify(text)} && !b.disabled);
    if (!button) return false;
    button.click();
    return true;
  })()`);

async function clickWhenReady(chrome, scope, text) {
  for (let i = 0; i < 100; i++) {
    if (await clickButton(chrome, scope, text)) return;
    await sleep(100);
  }
  throw new Error(`no enabled "${text}" button in ${scope}`);
}

const DIALOG = '[role="alertdialog"]';
const TOAST = '[data-sonner-toast]';

async function rebootDialog(chrome, base, vp, shoot) {
  await chrome.open(`${base}/#/system`, vp);
  await clickWhenReady(chrome, 'main', 'Restart router');
  await waitFor(chrome, `!!document.querySelector('${DIALOG}')`, 'reboot dialog');
  await sleep(300);
  await shoot('system-reboot-dialog');
}

async function apDisableFlow(chrome, base, vp, shoot) {
  await chrome.open(`${base}/#/wifi`, vp);
  await clickWhenReady(chrome, 'main', 'Disable');
  await waitFor(chrome, `!!document.querySelector('${DIALOG}')`, 'AP dialog');
  await sleep(300);
  await shoot('wifi-ap-disable-dialog');
  await clickWhenReady(chrome, DIALOG, 'Disable');
  await waitFor(chrome, `!!document.querySelector('${TOAST}')`, 'pending toast');
  await sleep(600);
  await shoot('wifi-pending-toast');
}

async function scheduleLockout(chrome, base, vp, shoot) {
  await chrome.open(`${base}/#/wifi`, vp);
  await clickWhenReady(chrome, 'main', 'Add schedule');
  await waitFor(chrome, `!!document.querySelector('[role="dialog"]')`, 'schedule dialog');
  await sleep(300);
  await shoot('wifi-schedule-dialog');
  await clickWhenReady(chrome, '[role="dialog"]', 'Save');
  await waitFor(chrome, `!!document.querySelector('${DIALOG}')`, 'lockout dialog');
  await sleep(300);
  await shoot('wifi-schedule-lockout-dialog');
}

/** The navigation sheet below md, opened on a grouped child page so its group shows expanded. */
async function navSheet(chrome, base, vp, shoot) {
  if (!vp.mobile) return;
  await chrome.open(`${base}/#/network/dhcp`, vp);
  await waitFor(
    chrome,
    `!!document.querySelector('header button[aria-controls="sidebar"]')`,
    'menu',
  );
  await chrome.evaluate(`document.querySelector('header button[aria-controls="sidebar"]').click()`);
  await waitFor(chrome, `!!document.querySelector('[data-slot="sheet-content"] nav')`, 'nav sheet');
  await sleep(500);
  await shoot('nav-sheet');
}

async function resultToast(chrome, base, vp, shoot) {
  await chrome.open(`${base}/#/vpn`, vp);
  await chrome.evaluate(`(() => {
    const forms = [...document.querySelectorAll('main form')];
    const form = forms[forms.length - 1];
    const set = (el, value) => { el.value = value; el.dispatchEvent(new Event('input', { bubbles: true })); };
    set(form.querySelector('input'), 'Office');
    set(form.querySelector('textarea'), '[Interface]');
    form.requestSubmit();
  })()`);
  await waitFor(chrome, `!!document.querySelector('${TOAST}')`, 'result toast');
  await sleep(600);
  await shoot('vpn-error-toast');
}

/** Runs every overlay flow at each viewport and scheme; `out(name, vp, scheme)` names the file. */
export async function shootOverlays(chrome, base, viewports, schemes, out) {
  const { identifier } = await chrome.send('Page.addScriptToEvaluateOnNewDocument', {
    source: stubFetch([
      ['POST', /^\/api\/v1\/wifi\/aps\/[^/]+$/, 202, PENDING_TXN],
      ['GET', /^\/api\/v1\/transactions\/screenshot-txn$/, 200, PENDING_TXN],
      ['POST', /^\/api\/v1\/vpn\/profiles$/, 422, { code: 'invalid_profile' }],
      ['PUT', /^\/api\/v1\/wifi\/schedule$/, 409, { code: 'wifi_lockout_risk' }],
    ]),
  });
  try {
    for (const flow of [rebootDialog, apDisableFlow, scheduleLockout, resultToast, navSheet]) {
      for (const viewport of viewports) {
        for (const scheme of schemes) {
          await flow(chrome, base, { ...viewport, scheme }, async (name) => {
            const path = out(name, viewport.width, scheme);
            await chrome.screenshot(path);
            console.log(`wrote ${path}`);
          });
        }
      }
    }
  } finally {
    await chrome.send('Page.removeScriptToEvaluateOnNewDocument', { identifier });
  }
}
