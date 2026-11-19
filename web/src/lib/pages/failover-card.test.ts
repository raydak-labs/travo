import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FailoverStatus } from '../api/gen';
import { failoverRefusal, fetchFailover, moveUplink, toggleUplink, warningText } from '../failover';
import type { LoadStatus } from '../ui/loader.svelte';
import FailoverCard from './FailoverCard.svelte';

const { createLoader, fo } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  fo: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as FailoverStatus | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const base: FailoverStatus = {
  enabled: false,
  uplinks: ['wan', 'wwan'],
  installed: false,
  running: false,
  active: false,
  blocked: false,
  states: [
    { name: 'wan', up: false },
    { name: 'wwan', up: true },
    { name: 'usbtether', up: false },
  ],
  warnings: [],
};

function card(data: Partial<FailoverStatus>) {
  fo.data = { ...base, ...data };
  return render(FailoverCard).body.replace(/<!--.*?-->/g, '');
}

describe('Failover card', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockReturnValue(fo);
  });

  it('shows the saved order with liveness and offers to enable', () => {
    const body = card({});
    expect(createLoader).toHaveBeenCalledWith(fetchFailover, 5000);
    expect(body).toContain('Off');
    expect(body.indexOf('Ethernet (wan)')).toBeLessThan(body.indexOf('WiFi uplink (wwan)'));
    expect(body).toContain('Add USB tethering');
    expect(body).toContain('Enabling installs the mwan3 package');
    expect(body).toContain('Enable failover');
    expect(body).not.toContain('Disable failover');
  });

  it('shows a running failover with warnings and both actions', () => {
    const body = card({
      enabled: true,
      installed: true,
      running: true,
      active: true,
      warnings: [{ section: 'wan', reason: 'adopted' }],
    });
    expect(body).toContain('On');
    expect(body).toContain('mwan3 section wan was taken over');
    expect(body).toContain('Apply order');
    expect(body).toContain('Disable failover');
  });

  it('flags an enabled failover that is not running', () => {
    expect(card({ enabled: true, installed: true })).toContain('Enabled, not running');
  });
});

describe('failover helpers', () => {
  it('moves and toggles uplinks', () => {
    expect(moveUplink(['wan', 'wwan', 'usbtether'], 2, -1)).toEqual(['wan', 'usbtether', 'wwan']);
    expect(moveUplink(['wan', 'wwan'], 0, -1)).toEqual(['wan', 'wwan']);
    expect(toggleUplink(['wan', 'wwan'], 'usbtether')).toEqual(['wan', 'wwan', 'usbtether']);
    expect(toggleUplink(['wan', 'wwan'], 'wan')).toEqual(['wwan']);
  });

  it('maps refusals and warnings', () => {
    expect(failoverRefusal('failover_offline').error).toContain('working internet connection');
    expect(failoverRefusal('failover_tailscale_conflict').error).toContain(
      'turn off its autostart',
    );
    expect(failoverRefusal('failover_no_change').message).toBeDefined();
    expect(failoverRefusal('x').error).toContain('Enabling again retries it');
    expect(warningText({ section: 'travo_m_wwan', reason: 'stale_edited' })).toContain(
      'kept, unused',
    );
  });
});
