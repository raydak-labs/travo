import { describe, expect, it } from 'vitest';
import { m } from '../paraglide/messages.js';
import { firewallRefusal, tailscaleError, tailscalePhase } from './tailscale';

describe('tailscalePhase', () => {
  it.each([
    [{ installed: false, running: false, backend_state: '' }, 'absent'],
    [{ installed: true, running: false, backend_state: '' }, 'stopped'],
    [{ installed: true, running: true, backend_state: 'NeedsLogin' }, 'login'],
    [{ installed: true, running: true, backend_state: 'NeedsMachineAuth' }, 'login'],
    [{ installed: true, running: true, backend_state: 'Running' }, 'connected'],
    [{ installed: true, running: true, backend_state: 'Stopped' }, 'off'],
    [{ installed: true, running: true, backend_state: 'NoState' }, 'off'],
    [{ installed: true, running: true, backend_state: 'Starting' }, 'starting'],
  ] as const)('%j -> %s', (st, want) => {
    expect(tailscalePhase(st)).toBe(want);
  });
});

describe('tailscaleError', () => {
  it('maps stable codes, never message text', () => {
    expect(tailscaleError('tailscale_exit_node_conflict')).toBe(m.tailscale_err_exit_conflict());
    expect(tailscaleError('tailscale_exit_node_offline')).toBe(m.tailscale_err_exit_offline());
    expect(tailscaleError('tailscale_no_space')).toBe(m.tailscale_err_no_space());
    expect(tailscaleError(undefined)).toBe(m.tailscale_err_failed());
  });

  it('treats an unchanged firewall as information', () => {
    expect(firewallRefusal('tailscale_no_change')).toEqual({ message: m.tailscale_fw_no_change() });
    expect(firewallRefusal('tailscale_firewall_conflict')).toEqual({
      error: m.tailscale_fw_conflict(),
    });
    expect(firewallRefusal('transaction_pending').error).toBeTruthy();
  });
});
