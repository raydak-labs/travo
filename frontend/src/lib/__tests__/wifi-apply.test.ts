import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { apiClient } from '../api-client';
import { confirmWifiApply, finalizeWifiMutation } from '../wifi-apply';

describe('wifi-apply', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('confirms immediately when the router is reachable', async () => {
    const spy = vi
      .spyOn(apiClient, 'post')
      .mockResolvedValueOnce({ status: 'ok' });

    await expect(confirmWifiApply('token-1', 1, 0)).resolves.toBeUndefined();
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('retries confirmation until it succeeds', async () => {
    vi.useFakeTimers();
    const spy = vi
      .spyOn(apiClient, 'post')
      .mockRejectedValueOnce(new Error('network down'))
      .mockResolvedValueOnce({ status: 'ok' });

    const promise = confirmWifiApply('token-2', 1, 10);
    await vi.runAllTimersAsync();

    await expect(promise).resolves.toBeUndefined();
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('finalizes pending wifi mutations by confirming their apply token', async () => {
    const confirmSpy = vi
      .spyOn(apiClient, 'post')
      .mockResolvedValueOnce({ status: 'ok' });

    const response = await finalizeWifiMutation(
      Promise.resolve({
        status: 'ok',
        apply: { pending: true, token: 'token-3', rollback_timeout_seconds: 1 },
      }),
    );

    expect(response.status).toBe('ok');
    expect(confirmSpy).toHaveBeenCalledWith('/api/v1/wifi/apply/confirm', { token: 'token-3' });
  });
});
