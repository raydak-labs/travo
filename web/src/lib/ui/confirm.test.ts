import { describe, expect, it } from 'vitest';
import { createConfirm } from './confirm.svelte';
import { toastTiming } from './transaction-toast.svelte';

const request = { title: 'Restart?', description: 'Sessions end.', action: 'Restart' };

describe('createConfirm', () => {
  it('resolves true only when the action is picked', async () => {
    const confirm = createConfirm();
    const answer = confirm.ask(request);
    expect(confirm.request).toEqual(request);
    confirm.answer(true);
    await expect(answer).resolves.toBe(true);
    expect(confirm.request).toBeUndefined();
  });

  it('resolves false on cancel', async () => {
    const confirm = createConfirm();
    const answer = confirm.ask(request);
    confirm.answer(false);
    await expect(answer).resolves.toBe(false);
  });

  it('declines an open question when a newer one replaces it', async () => {
    const confirm = createConfirm();
    const first = confirm.ask(request);
    const second = confirm.ask({ ...request, title: 'Other?' });
    await expect(first).resolves.toBe(false);
    expect(confirm.request?.title).toBe('Other?');
    confirm.answer(true);
    await expect(second).resolves.toBe(true);
  });
});

describe('toastTiming', () => {
  it('keeps open phases until they settle, without swipe', () => {
    for (const phase of [
      { phase: 'applying', text: 'x' },
      { phase: 'pending', deadline: 1, settling: false },
      { phase: 'restoring', text: 'x' },
    ] as const) {
      expect(toastTiming(phase)).toEqual({ duration: Infinity, dismissible: false });
    }
  });

  it('keeps errors until dismissed and fades the other results', () => {
    expect(toastTiming({ phase: 'done', tone: 'error', text: 'x' })).toEqual({
      duration: Infinity,
      dismissible: true,
    });
    expect(toastTiming({ phase: 'done', tone: 'success', text: 'x' }).duration).toBe(5000);
  });
});
