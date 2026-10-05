import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { apiClient, ApiError } from '../api-client';
import {
  confirmDeadlineMs,
  confirmWifiApply,
  DEFAULT_PROBE_BUDGET_SECONDS,
  finalizeWifiMutation,
  isTerminalApplyStatus,
  isWirelessNotVerifiedError,
  isWirelessStatusUnreadableError,
  PROBE_SAFETY_MARGIN_MS,
  WifiApplyRolledBackError,
  WifiApplyUnverifiedError,
} from '../wifi-apply';

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

  // services.ErrWirelessNotUp arrives as a 500, so the retry loop used to keep
  // polling for the full 30s and then report a generic rollback timeout instead
  // of telling the operator the router already reverted the change.
  it('stops retrying when the router reports the wireless apply was not verified', async () => {
    const spy = vi
      .spyOn(apiClient, 'post')
      .mockRejectedValue(
        new ApiError(
          500,
          'wireless apply not verified: enabled access point(s) did not come up: default_radio1',
        ),
      );

    await expect(confirmWifiApply('token-6', 30, 1)).rejects.toBeInstanceOf(
      WifiApplyRolledBackError,
    );
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('recognises the not-verified error by message, whatever the status', () => {
    expect(
      isWirelessNotVerifiedError(new ApiError(500, 'wireless apply not verified: sections x')),
    ).toBe(true);
    expect(isWirelessNotVerifiedError(new Error('token is required'))).toBe(false);
    expect(isWirelessNotVerifiedError(undefined)).toBe(false);
  });

  // "I cannot read netifd's answer" is not "your access point did not come up",
  // and the rollback window is still OPEN when it happens. Telling the operator
  // the router rolled back asserts something that has not happened yet. Both
  // messages below are what the backend actually sends today
  // (services.ErrWirelessUnverifiable).
  it.each([
    'wireless apply could not be verified: network.wireless status could not be read: ' +
      'the ubus call did not complete: ubus connection reset by peer',
    'wireless apply could not be verified: network.wireless status could not be read: ' +
      'radio radio0 reports no "up" flag',
  ])('separates an unreadable device answer (%s) from a confirmed rollback', async (message) => {
    vi.spyOn(apiClient, 'post').mockRejectedValue(new ApiError(500, message));

    const assertion = expect(confirmWifiApply('token-shape', 30, 1)).rejects.toBeInstanceOf(
      WifiApplyUnverifiedError,
    );
    await assertion;
  });

  it('does not read an unreadable status as a rollback, and does not claim one', () => {
    const shape = new ApiError(
      500,
      'wireless apply could not be verified: network.wireless status could not be read',
    );
    expect(isWirelessNotVerifiedError(shape)).toBe(false);
    expect(isWirelessStatusUnreadableError(shape)).toBe(true);
    expect(isWirelessStatusUnreadableError(new Error('token is required'))).toBe(false);

    const unverified = new WifiApplyUnverifiedError();
    expect(unverified.message.toLowerCase()).not.toContain('rolled back');
    expect(unverified.message.toLowerCase()).toContain('could not verify');
  });

  it('still reports a plain rollback timeout when the router never answers', async () => {
    vi.useFakeTimers();
    vi.spyOn(apiClient, 'post').mockRejectedValue(new Error('network down'));

    const assertion = expect(confirmWifiApply('token-7', 1, 10)).rejects.toThrow(
      /rollback timeout/,
    );
    await vi.runAllTimersAsync();

    await assertion;
  });
});

// apply.probe_budget_seconds is how long ONE confirm call can block on the
// device while netifd brings the new interfaces up (12 s today: ~10 s of sleeps
// covering the measured ACS + link-up, plus the ubus round-trips the
// per-interface fallback makes). The client
// used to probe until the full rollback timeout, so a probe started near the
// deadline was answered only after rpcd had already rolled back and the
// operator saw a session failure instead of a clean rollback.
describe('confirm probe budget', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('subtracts the probe budget and a margin from the rollback window', () => {
    expect(confirmDeadlineMs(30, 4, 1_000)).toBe(1_000 + 30_000 - 4_000 - PROBE_SAFETY_MARGIN_MS);
  });

  it('shortens the deadline as the declared probe budget grows', () => {
    // A probe that may block longer must be issued earlier.
    expect(confirmDeadlineMs(30, 1, 0)).toBeGreaterThan(confirmDeadlineMs(30, 8, 0));
  });

  // An older backend omits the field; the deadline must still be a usable
  // finite instant in the future, not NaN and not in the past.
  it.each([undefined, Number.NaN, 0, -5])(
    'falls back to a default budget instead of a NaN or past deadline for %p',
    (budget) => {
      const deadline = confirmDeadlineMs(30, budget, 1_000);
      expect(Number.isFinite(deadline)).toBe(true);
      expect(deadline).toBe(
        1_000 + 30_000 - DEFAULT_PROBE_BUDGET_SECONDS * 1000 - PROBE_SAFETY_MARGIN_MS,
      );
    },
  );

  it('never lets the budget consume more than half the rollback window', () => {
    // A 1 s window: the reservation is capped at 500 ms, so the deadline is
    // half the window away and the caller still gets an instant to probe at.
    expect(confirmDeadlineMs(1, 4, 0)).toBe(500);
  });

  // The deadline must leave the final probe's own blocking time AND real
  // headroom before rpcd fires. A 500 ms margin left the last probe answered
  // 0.5 s before the rollback, so any device that took a second longer turned a
  // clean rollback into a confusing failure.
  it('leaves at least a second of headroom before the rollback with the default budget', () => {
    const rollbackMs = 30_000;
    const lastProbeEndsAt =
      confirmDeadlineMs(30, DEFAULT_PROBE_BUDGET_SECONDS, 0) + DEFAULT_PROBE_BUDGET_SECONDS * 1000;
    expect(rollbackMs - lastProbeEndsAt).toBeGreaterThanOrEqual(1_000);
  });

  // The fallback budget is used when the backend does not publish one, so it
  // has to satisfy the same cap the published value does: a device budget too
  // big for half the 30 s window would be truncated silently and the probe
  // would be issued too late.
  it('keeps the fallback budget inside half the rollback window', () => {
    const rollbackWindowMs = 30_000;
    const reserved = DEFAULT_PROBE_BUDGET_SECONDS * 1000 + PROBE_SAFETY_MARGIN_MS;
    expect(reserved).toBeLessThan(rollbackWindowMs / 2);
  });

  it('never issues a probe that could still be in flight when rpcd rolls back', async () => {
    vi.useFakeTimers();
    const rollbackMs = 30_000;
    const probeMs = 4_000;
    const probeStarted: number[] = [];

    vi.spyOn(apiClient, 'post').mockImplementation(async () => {
      probeStarted.push(Date.now());
      await new Promise((resolve) => setTimeout(resolve, probeMs));
      throw new Error('network down');
    });

    const start = Date.now();
    const assertion = expect(
      confirmWifiApply('token-8', rollbackMs / 1000, 1500, probeMs / 1000),
    ).rejects.toThrow(/rollback timeout/);
    await vi.runAllTimersAsync();
    await assertion;

    expect(probeStarted.length).toBeGreaterThan(1);
    for (const issuedAt of probeStarted) {
      // The probe is answered up to probeMs later; it must be back before
      // rpcd's rollback fires at start + rollbackMs.
      expect(issuedAt + probeMs).toBeLessThanOrEqual(start + rollbackMs);
    }
  });

  it('passes the response probe budget on to the confirm loop', async () => {
    vi.useFakeTimers();
    // Last confirm probe issued, as milliseconds since the apply was answered.
    const lastProbeAfterApply = async (probeBudgetSeconds: number): Promise<number> => {
      const starts: number[] = [];
      vi.spyOn(apiClient, 'post').mockImplementation(async () => {
        starts.push(Date.now());
        throw new Error('network down');
      });

      const start = Date.now();
      const assertion = expect(
        finalizeWifiMutation(
          Promise.resolve({
            status: 'ok',
            apply: {
              pending: true,
              token: 'token-9',
              rollback_timeout_seconds: 30,
              probe_budget_seconds: probeBudgetSeconds,
            },
          }),
        ),
      ).rejects.toThrow(/rollback timeout/);
      await vi.runAllTimersAsync();
      await assertion;
      return starts[starts.length - 1] - start;
    };

    const withBudget = await lastProbeAfterApply(8);
    const withoutBudget = await lastProbeAfterApply(1);

    expect(withBudget).toBeLessThan(withoutBudget);
    expect(withBudget).toBeLessThan(30_000);
  });

  // The envelope field is optional on the wire: an older backend must still be
  // confirmable instead of failing before a single probe is sent.
  it('confirms normally when the response omits probe_budget_seconds', async () => {
    const spy = vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ status: 'ok' });

    await expect(
      finalizeWifiMutation(
        Promise.resolve({
          status: 'ok',
          apply: { pending: true, token: 'token-10', rollback_timeout_seconds: 30 },
        }),
      ),
    ).resolves.toMatchObject({ status: 'ok' });
    expect(spy).toHaveBeenCalledTimes(1);
  });
});
