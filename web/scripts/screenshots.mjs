// Writes PNG screenshots of the UI at mobile and desktop widths in light and dark scheme.
// Expects the daemon and `vite preview` already running (see `mise run screenshots`).
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { launchChrome } from './cdp.mjs';
import { shootOverlays } from './screenshots-overlays.mjs';

const BASE_URL = process.env.SCREENSHOT_URL ?? `http://localhost:${process.env.WEB_PORT ?? 18092}`;
// Login and setup are only reachable signed out; the rest needs the session (see main).
const SIGNED_IN_ROUTES = [
  '/',
  '/internet',
  '/wifi',
  '/vpn',
  '/vpn/wireguard',
  '/vpn/tailscale',
  '/clients',
  '/services',
  '/services/adguard',
  '/network/lan',
  '/network/port-forwards',
  '/network/ipv6',
  '/network/diagnostics',
  '/system',
  '/system/hardware',
  '/system/logs',
  '/system/firmware',
  '/system/access',
];
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

// A fresh mock daemon has no history yet. Stub one day of 10 min buckets (one restart, an
// uplink switch, a degraded file) so the history card's charts are visible; no router involved.
const FAKE_HISTORY = `(() => {
  const end = Date.UTC(2026, 10, 18, 12) / 1000;
  const points = [];
  for (let i = 0; i < 144; i++) {
    const at = end - (143 - i) * 600;
    const wave = Math.sin(i / 9) + 1;
    const uplinks = i === 60 ? [] : [{ name: i < 100 ? 'wwan' : 'usbtether',
      rx_bytes: Math.round(wave * 40e6 + (i % 7) * 3e6), tx_bytes: Math.round(wave * 6e6) }];
    points.push({ at, uptime_seconds: i < 61 ? 86400 + i * 600 : (i - 61) * 600 + 120,
      clients: 2 + Math.round(wave * 3), uplinks });
  }
  const body = { interval_seconds: 600, retention_hours: 168, persistence: 'degraded', points };
  const real = window.fetch;
  window.fetch = (input, init) => {
    const url = new URL(typeof input === 'string' ? input : input.url, location.href);
    if (url.pathname === '/api/v1/stats/history') return Promise.resolve(Response.json(body));
    return real(input, init);
  };
})()`;

async function shootHistory(chrome) {
  const { identifier } = await chrome.send('Page.addScriptToEvaluateOnNewDocument', {
    source: FAKE_HISTORY,
  });
  for (const vp of VIEWPORTS) {
    for (const scheme of SCHEMES) {
      await chrome.open(`${BASE_URL}/#/`, { ...vp, scheme });
      await chrome.evaluate(
        `document.querySelector('[aria-label="Time range"]')?.scrollIntoView(); window.scrollBy(0, -72)`,
      );
      const out = `${OUT_DIR}dashboard-history-${vp.width}-${scheme}.png`;
      await chrome.screenshot(out);
      console.log(`wrote ${out}`);
    }
  }
  await chrome.send('Page.removeScriptToEvaluateOnNewDocument', { identifier });
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
    await shootHistory(chrome);
    await shootOverlays(
      chrome,
      BASE_URL,
      VIEWPORTS,
      SCHEMES,
      (name, width, scheme) => `${OUT_DIR}${name}-${width}-${scheme}.png`,
    );
  } finally {
    chrome.close();
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exitCode = 1;
});
