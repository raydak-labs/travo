// Browser check of the auth flow against `--mock --mock-firstrun` (see `mise run e2e:auth`):
// setup -> dashboard -> reload -> log out -> wrong password -> login. Exits non-zero on any failed step.
import { launchChrome } from './cdp.mjs';

const BASE_URL = process.env.E2E_URL ?? `http://localhost:${process.env.WEB_PORT ?? 18092}`;
const PASSWORD = 'e2e-password-1';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const chrome = await launchChrome();

/** Polls a page expression until truthy; the step name makes a failure readable. */
async function waitFor(step, expression, timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    if (await chrome.evaluate(expression)) return;
    if (Date.now() > deadline) {
      const where = await chrome.evaluate('location.href');
      throw new Error(`step failed: ${step} (at ${where})`);
    }
    await sleep(100);
  }
}

/** Types into a password input the way a user would, so the framework sees an input event. */
const fill = (id, value) =>
  chrome.evaluate(`(() => {
    const el = document.getElementById(${JSON.stringify(id)});
    el.value = ${JSON.stringify(value)};
    el.dispatchEvent(new Event('input', { bubbles: true }));
  })()`);

const submit = () => chrome.evaluate(`document.querySelector('form').requestSubmit()`);
const hashIs = (hash) => `location.hash === ${JSON.stringify(hash)}`;
const onDashboard = `(location.hash === '#/' || location.hash === '') && !!document.querySelector('dl')`;

async function main() {
  // open() waits for the exact URL, which the guard redirect changes: navigate directly.
  await chrome.send('Page.enable');
  await chrome.send('Page.navigate', { url: `${BASE_URL}/#/` });

  await waitFor('# / lands on setup', hashIs('#/setup'));
  await waitFor('setup form rendered', `!!document.getElementById('setup-password')`);
  await fill('setup-password', PASSWORD);
  await fill('setup-confirm', PASSWORD);
  await submit();
  await waitFor('setting the password lands on the dashboard', onDashboard);

  await chrome.send('Page.reload');
  await waitFor('reload keeps the dashboard', onDashboard);

  await chrome.evaluate(`document.querySelector('header button[aria-label="Log out"]').click()`);
  await waitFor(
    'log out lands on login',
    `${hashIs('#/login')} && !!document.getElementById('login-password')`,
  );

  await fill('login-password', 'wrong-password');
  await submit();
  await waitFor(
    'wrong password shows an error',
    `document.getElementById('login-password-error')?.textContent.trim() === 'Incorrect password.'`,
  );

  await fill('login-password', PASSWORD);
  await submit();
  await waitFor('correct password lands on the dashboard', onDashboard);

  // returnTo: a logged-out visit to #/system comes back there after login, not to the dashboard.
  await chrome.evaluate(`document.querySelector('header button[aria-label="Log out"]').click()`);
  await waitFor('log out again lands on login', hashIs('#/login'));
  await chrome.send('Page.navigate', { url: `${BASE_URL}/#/system` });
  await waitFor(
    '#/system while logged out shows login',
    `${hashIs('#/login')} && !!document.getElementById('login-password')`,
  );
  await fill('login-password', PASSWORD);
  await submit();
  await waitFor('login returns to #/system', hashIs('#/system'));
  console.log('e2e auth: ok');
}

try {
  await main();
} catch (err) {
  console.error(err.message);
  process.exitCode = 1;
} finally {
  chrome.close();
}
