import { beforeEach, describe, expect, it, vi } from 'vitest';

const { changeTailscale, notify } = vi.hoisted(() => ({
  changeTailscale: vi.fn(),
  notify: { dismiss: vi.fn(), error: vi.fn(), progress: vi.fn(), success: vi.fn() },
}));
vi.mock('./api/tailscale', () => ({ changeTailscale }));
vi.mock('./ui/notify', () => ({ notify }));

import { createTailscaleControl } from './tailscale-control.svelte';

const hang = () => new Promise(() => {});

describe('Tailscale control', () => {
  beforeEach(() => vi.clearAllMocks());

  it('reports success, then refreshes, and leaves the result toast open', async () => {
    changeTailscale.mockResolvedValue({ status: { running: true } });
    const refresh = vi.fn();
    const control = createTailscaleControl(refresh);
    await control.change('on');
    expect(changeTailscale).toHaveBeenCalledWith('on', expect.any(AbortSignal), undefined);
    expect(notify.progress).toHaveBeenCalledWith('Applying…', 'tailscale');
    expect(notify.success).toHaveBeenCalledWith(expect.any(String), 'tailscale');
    expect(refresh).toHaveBeenCalledOnce();
    expect(control.pending).toBeUndefined();
    control.stop();
    expect(notify.dismiss).not.toHaveBeenCalled();
  });

  it('keeps a refusal toast open after the page is left', async () => {
    changeTailscale.mockResolvedValue({ error: 'tailscaled did not stop' });
    const control = createTailscaleControl(vi.fn());
    await control.change('off');
    control.stop();
    expect(notify.error).toHaveBeenCalledWith('tailscaled did not stop', 'tailscale');
    expect(notify.dismiss).not.toHaveBeenCalled();
  });

  it('says it is installing while the package installs', async () => {
    changeTailscale.mockReturnValue(hang());
    const control = createTailscaleControl(vi.fn());
    void control.change('install');
    expect(notify.progress).toHaveBeenCalledWith('Installing Tailscale…', 'tailscale');
    expect(control.pending).toBe('install');
  });

  it('passes the settings through with a settings change', async () => {
    changeTailscale.mockResolvedValue({ status: {} });
    const settings = { exit_node: '', advertise_lan: true, ssh: false };
    await createTailscaleControl(vi.fn()).change('settings', settings);
    expect(changeTailscale).toHaveBeenCalledWith('settings', expect.any(AbortSignal), settings);
  });

  it('aborts the request and closes its progress toast when the page is left mid-request', () => {
    let signal: AbortSignal | undefined;
    changeTailscale.mockImplementation((_: string, s: AbortSignal) => {
      signal = s;
      return hang();
    });
    const refresh = vi.fn();
    const control = createTailscaleControl(refresh);
    void control.change('on');
    control.stop();
    expect(signal?.aborted).toBe(true);
    expect(notify.dismiss).toHaveBeenCalledWith('tailscale');
    expect(refresh).not.toHaveBeenCalled();
  });

  it('ignores a second change while the first is in flight, and any change after stop', () => {
    changeTailscale.mockReturnValue(hang());
    const busy = createTailscaleControl(vi.fn());
    void busy.change('on');
    void busy.change('off');
    expect(changeTailscale).toHaveBeenCalledTimes(1);

    const stopped = createTailscaleControl(vi.fn());
    stopped.stop();
    void stopped.change('on');
    expect(changeTailscale).toHaveBeenCalledTimes(1);
  });
});
