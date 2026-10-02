import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { apiClient, ApiError } from '../api-client';
import { confirmWifiApply, finalizeWifiMutation, isTerminalApplyStatus } from '../wifi-apply';

describe('wifi-apply', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('confirms immediately when the router is reachable', async () => {
    const spy = vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ status: 'ok' });

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
    const confirmSpy = vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ status: 'ok' });

    const response = await finalizeWifiMutation(
      Promise.resolve({
        status: 'ok',
        apply: { pending: true, token: 'token-3', rollback_timeout_seconds: 1 },
      }),
    );

    expect(response.status).toBe('ok');
    expect(confirmSpy).toHaveBeenCalledWith('/api/v1/wifi/apply/confirm', { token: 'token-3' });
  });

  // A 401 used to be retried ~20 times over 30s, clearing the session token
  // and re-issuing location.assign('/login') on every attempt.
  it.each([400, 401, 403, 404, 409, 422])(
    'stops immediately on a terminal %i and surfaces the device error',
    async (status) => {
      const spy = vi
        .spyOn(apiClient, 'post')
        .mockRejectedValue(new ApiError(status, `Request failed with status ${status}`));

      await expect(confirmWifiApply('token-4', 30, 1)).rejects.toThrow(
        `Request failed with status ${status}`,
      );
      expect(spy).toHaveBeenCalledTimes(1);
    },
  );

  it.each([408, 429, 500, 502, 503])('keeps retrying a transient %i', async (status) => {
    vi.useFakeTimers();
    const spy = vi
      .spyOn(apiClient, 'post')
      .mockRejectedValueOnce(new ApiError(status, 'temporary'))
      .mockResolvedValueOnce({ status: 'ok' });

    const promise = confirmWifiApply('token-5', 1, 10);
    await vi.runAllTimersAsync();

    await expect(promise).resolves.toBeUndefined();
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('classifies terminal statuses', () => {
    expect(isTerminalApplyStatus(400)).toBe(true);
    expect(isTerminalApplyStatus(401)).toBe(true);
    expect(isTerminalApplyStatus(403)).toBe(true);
    expect(isTerminalApplyStatus(408)).toBe(false);
    expect(isTerminalApplyStatus(429)).toBe(false);
    expect(isTerminalApplyStatus(500)).toBe(false);
    expect(isTerminalApplyStatus(200)).toBe(false);
  });
});
