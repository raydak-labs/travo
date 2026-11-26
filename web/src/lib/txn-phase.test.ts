import { describe, expect, it } from 'vitest';
import type { TxnBody } from './api/gen';
import { controlPhase } from './vpn-tunnel.svelte';
import { joinPhase, reduceJoin } from './wifi-join';

// Both guarded flows (WiFi join/AP edits and VPN toggles) feed the same transaction toast.
const txn = (state: TxnBody['state']): TxnBody => ({
  id: '1234567890abcdef',
  state,
  started: '2026-11-13T22:47:16Z',
  configs: ['wireless'],
  summary: 'wifi: join Uplink',
  window_seconds: 60,
});

describe('joinPhase', () => {
  it('shows progress, then the pending window with its deadline', () => {
    expect(joinPhase({ state: 'idle' }, false)).toEqual({ phase: 'idle' });
    expect(joinPhase({ state: 'joining', ssid: 'Cafe' }, false)).toEqual({
      phase: 'applying',
      text: 'Connecting to Cafe, this can take up to 30 seconds',
    });
    const armed = reduceJoin(
      { state: 'joining', ssid: 'Cafe' },
      { type: 'transaction', now: 1000, transaction: txn('pending') },
    );
    expect(joinPhase(armed, true)).toEqual({ phase: 'pending', deadline: 61_000, settling: true });
  });

  it('reports each outcome with its tone', () => {
    const settled = (state: TxnBody['state']) =>
      joinPhase({ state, transaction: txn(state), deadline: 0 }, false);
    expect(settled('confirmed')).toMatchObject({ phase: 'done', tone: 'success' });
    expect(settled('rolled_back')).toMatchObject({ phase: 'done', tone: 'info' });
    expect(settled('rollback_failed')).toMatchObject({ phase: 'done', tone: 'error' });
    expect(settled('restoring')).toMatchObject({ phase: 'restoring' });
    expect(joinPhase({ state: 'failed', code: 'wifi_lockout_risk' }, false)).toEqual({
      phase: 'done',
      tone: 'error',
      text: 'This change would disconnect you from the router.',
    });
  });
});

describe('controlPhase', () => {
  const view = {
    applying: false,
    settling: false,
    transaction: undefined as TxnBody | undefined,
    deadline: 0,
    message: '',
    error: '',
  };

  it('maps a guarded toggle to the toast phase', () => {
    expect(controlPhase(view, 'Applying')).toEqual({ phase: 'idle' });
    expect(controlPhase({ ...view, applying: true }, 'Applying')).toEqual({
      phase: 'applying',
      text: 'Applying',
    });
    const open = { ...view, transaction: txn('pending'), deadline: 5, settling: true };
    expect(controlPhase(open, 'x')).toEqual({ phase: 'pending', deadline: 5, settling: true });
    expect(controlPhase({ ...view, transaction: txn('restoring') }, 'x')).toMatchObject({
      phase: 'restoring',
    });
    expect(controlPhase({ ...view, error: 'Nope' }, 'x')).toEqual({
      phase: 'done',
      tone: 'error',
      text: 'Nope',
    });
    expect(
      controlPhase({ ...view, transaction: txn('confirmed'), message: 'Done' }, 'x'),
    ).toMatchObject({ tone: 'success' });
    expect(controlPhase({ ...view, message: 'Already on' }, 'x')).toMatchObject({ tone: 'info' });
  });
});
