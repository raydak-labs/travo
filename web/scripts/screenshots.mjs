// Writes PNG screenshots of the UI at mobile and desktop widths in light and dark scheme.
// Expects the daemon and `vite preview` already running (see `mise run screenshots`).
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { launchChrome } from './cdp.mjs';

const BASE_URL = process.env.SCREENSHOT_URL ?? `http://localhost:${process.env.WEB_PORT ?? 18092}`;
// Login and setup are only reachable signed out; the rest needs the session (see main).
const SIGNED_IN_ROUTES = ['/', '/wifi', '/system', '/system/tokens', '/vpn', '/services'];
const MOCK_PASSWORD = 'mock-password';
const VIEWPORTS = [
  { width: 390, height: 844, mobile: true },
  { width: 1280, height: 800, mobile: false },
];
const SCHEMES = ['light', 'dark'];

const OUT_DIR = fileURLToPath(new URL('../screenshots/', import.meta.url));

/** `/` becomes `dashboard`; other routes drop their slashes (`/system` -> `system`). */
export function routeName(route) {
  return route === '/' ? 'dashboard' : route.replace(/\//g, '');
}

export function screenshotPath(route, width, scheme) {
  return `${OUT_DIR}${routeName(route)}-${width}-${scheme}.png`;
}

async function shootAll(chrome, routes) {
  for (const route of routes) {
    for (const vp of VIEWPORTS) {
      for (const scheme of SCHEMES) {
        await chrome.open(`${BASE_URL}/#${route}`, { ...vp, scheme });
        const out = screenshotPath(route, vp.width, scheme);
        await chrome.screenshot(out);
        console.log(`wrote ${out}`);
      }
    }
  }
}

// The mock daemon here already has a password, so /setup would redirect to login. Make the page
// believe it is first run: only the setup-state read is faked, the form is the real one.
const FAKE_FIRST_RUN = `(() => {
  const real = window.fetch;
  window.fetch = (input, init) => {
    const url = new URL(typeof input === 'string' ? input : input.url, location.href);
    if (url.pathname === '/api/v1/setup') {
      return Promise.resolve(Response.json({ password_set: false }));
    }
    return real(input, init);
  };
})()`;

async function signIn(chrome) {
  await chrome.open(`${BASE_URL}/#/login`, { width: 1280, height: 800 });
  await chrome.evaluate(`(() => {
    const el = document.getElementById('login-password');
    el.value = ${JSON.stringify(MOCK_PASSWORD)};
    el.dispatchEvent(new Event('input', { bubbles: true }));
    document.querySelector('form').requestSubmit();
  })()`);
  for (let i = 0; i < 100; i++) {
    if (await chrome.evaluate(`location.hash === '#/' && !!document.querySelector('dl')`)) return;
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error('could not sign in to the mock daemon');
}

function fakeVpnStatus(state) {
  const status = {
    state,
    stale: false,
    observed_at: '2026-11-13T22:47:16Z',
    handshake_age_seconds: state === 'connected' ? 14 : null,
  };
  return `(() => {
    const real = window.fetch;
    window.fetch = (input, init) => {
      const url = new URL(typeof input === 'string' ? input : input.url, location.href);
      if (url.pathname === '/api/v1/vpn/status') {
        return Promise.resolve(Response.json(${JSON.stringify(status)}));
      }
      return real(input, init);
    };
  })()`;
}

// The mock device has no network config and therefore covers disabled through the real daemon.
// Render the other states locally by stubbing only their status response; no router is involved.
async function shootVpnStates(chrome) {
  for (const state of ['configured', 'enabled_not_up', 'no_handshake', 'connected']) {
    const { identifier } = await chrome.send('Page.addScriptToEvaluateOnNewDocument', {
      source: fakeVpnStatus(state),
    });
    // The dashboard quick action shares the VPN page states; one ready-to-enable shot covers it.
    const pages = state === 'configured' ? ['vpn', 'dashboard'] : ['vpn'];
    for (const page of pages) {
      for (const vp of VIEWPORTS) {
        for (const scheme of SCHEMES) {
          await chrome.open(`${BASE_URL}/#/${page === 'dashboard' ? '' : page}`, { ...vp, scheme });
          const out = `${OUT_DIR}${page === 'vpn' ? 'vpn' : 'dashboard-vpn'}-${state}-${vp.width}-${scheme}.png`;
          await chrome.screenshot(out);
          console.log(`wrote ${out}`);
        }
      }
    }
    await chrome.send('Page.removeScriptToEvaluateOnNewDocument', { identifier });
  }
}

async function main() {
  mkdirSync(OUT_DIR, { recursive: true });
  const chrome = await launchChrome();
  try {
    await shootAll(chrome, ['/login']);
    const { identifier } = await chrome.send('Page.addScriptToEvaluateOnNewDocument', {
      source: FAKE_FIRST_RUN,
    });
    await shootAll(chrome, ['/setup']);
    await chrome.send('Page.removeScriptToEvaluateOnNewDocument', { identifier });
    await signIn(chrome);
    await shootAll(chrome, SIGNED_IN_ROUTES);
    await shootVpnStates(chrome);
  } finally {
    chrome.close();
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exitCode = 1;
});
