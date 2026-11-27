import { beforeEach, describe, expect, it, vi } from 'vitest';

const { changeTailscale, notify } = vi.hoisted(() => ({
  changeTailscale: vi.fn(),
  notify: { dismiss: vi.fn(), error: vi.fn(), progress: vi.fn(), success: vi.fn() },
}));
vi.mock('./api/tailscale', () => ({ changeTailscale }));
vi.mock('./ui/notify', () => ({ notify }));

import { createTailscaleToggle } from './tailscale-toggle.svelte';

describe('Tailscale toggle', () => {
  beforeEach(() => vi.clearAllMocks());

  it('keeps the result toast when its switch is rebuilt by the refresh', async () => {
    changeTailscale.mockResolvedValue({ status: { running: true } });
    const control = createTailscaleToggle(() => control.stop());
    await control.toggle(true);
    expect(changeTailscale).toHaveBeenCalledWith('on', expect.any(AbortSignal));
    expect(notify.success).toHaveBeenCalledWith(expect.any(String), 'tailscale');
    expect(notify.dismiss).not.toHaveBeenCalled();
  });

  it('keeps a refusal toast open after the refresh', async () => {
    changeTailscale.mockResolvedValue({ error: 'tailscaled did not stop' });
    const control = createTailscaleToggle(() => control.stop());
    await control.toggle(false);
    expect(notify.error).toHaveBeenCalledWith('tailscaled did not stop', 'tailscale');
    expect(notify.dismiss).not.toHaveBeenCalled();
  });

  it('aborts the request and closes the progress toast when its switch goes away mid-request', async () => {
    let signal: AbortSignal | undefined;
    changeTailscale.mockImplementation((_: string, s: AbortSignal) => {
      signal = s;
      return new Promise(() => {});
    });
    const refresh = vi.fn();
    const control = createTailscaleToggle(refresh);
    void control.toggle(true);
    expect(control.pending).toBe(true);
    control.stop();
    expect(signal?.aborted).toBe(true);
    expect(notify.dismiss).toHaveBeenCalledWith('tailscale');
    expect(refresh).not.toHaveBeenCalled();
  });

  it('ignores a second flip while the first is in flight', async () => {
    changeTailscale.mockReturnValue(new Promise(() => {}));
    const control = createTailscaleToggle(vi.fn());
    void control.toggle(true);
    void control.toggle(false);
    expect(changeTailscale).toHaveBeenCalledTimes(1);
  });
});
