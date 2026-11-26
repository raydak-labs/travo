import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
import Tailscale from './Tailscale.svelte';

const { createLoader } = vi.hoisted(() => ({ createLoader: vi.fn() }));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('./TailscaleOptions.svelte', () => ({ default: () => {} }));

const base = {
  installed: true,
  version: '1.104.1',
  running: true,
  busy: false,
  backend_state: 'Running',
  auth_url: '',
  self: {
    id: 'n',
    host_name: 'router',
    dns_name: 'router.tail.ts.net.',
    ips: ['100.64.0.1'],
    online: true,
  },
  magic_dns: { enabled: true, suffix: 'tail.ts.net' },
  exit_nodes: [],
  prefs: { exit_node: '', advertise_routes: [], ssh: false, accept_dns: false, want_running: true },
  health: [],
  settings: { exit_node: '', advertise_lan: false, ssh: false },
  firewall_in_sync: true,
};

function page(data: unknown) {
  const ready: LoadStatus = { state: 'ready' };
  createLoader.mockReturnValue({ status: ready, data, retry: vi.fn(), stop: vi.fn() });
  return render(Tailscale).body;
}

describe('Tailscale page', () => {
  beforeEach(() => vi.clearAllMocks());

  it('connected: the header carries the on switch and Log out', () => {
    const body = page(base);
    expect(switchState(body, 'tailscale-running')).toEqual({ checked: true, disabled: false });
    expect(body).toContain('Log out');
    expect(body).toContain('100.64.0.1');
  });

  it('off: the switch is off, the login is kept and no Log out is offered', () => {
    const body = page({ ...base, running: false, backend_state: '' });
    expect(switchState(body, 'tailscale-running')).toEqual({ checked: false, disabled: false });
    expect(body).not.toContain('Log out');
    expect(body).toContain('Your login is kept');
  });

  it('absent: Install instead of a switch', () => {
    const body = page({ ...base, installed: false, running: false, version: '' });
    expect(switchState(body, 'tailscale-running')).toBeNull();
    expect(body).toContain('Install');
  });
});
