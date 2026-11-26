import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  getSystemSettings,
  getWiFiAccessPoints,
  getWiFiSchedule,
  getWiFiScheduleStatus,
  setWiFiSchedule,
  type WiFiScheduleRule,
} from './gen';
import {
  daysLabel,
  emptyForm,
  fetchSchedule,
  formFrom,
  formProblem,
  lastRun,
  removeRule,
  runTime,
  saveSchedule,
  saveWithConsent,
  scheduleError,
  setRuleEnabled,
  upsertRule,
} from './wifi-schedule';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({
  getSystemSettings: vi.fn(),
  getWiFiAccessPoints: vi.fn(),
  getWiFiSchedule: vi.fn(),
  getWiFiScheduleStatus: vi.fn(),
  setWiFiSchedule: vi.fn(),
}));

afterEach(() => vi.resetAllMocks());

const reply = (data: unknown, status = 200) =>
  ({ data, response: new Response(null, { status }) }) as never;
const refused = (code: string, status = 409) =>
  ({ data: undefined, error: { code }, response: new Response(null, { status }) }) as never;

const ap = {
  band: '5g',
  enabled: true,
  guest: false,
  radio: 'radio1',
  security: 'protected',
} as const;
const rule: WiFiScheduleRule = {
  id: 'a1b2c3d4',
  section: 'default_radio1',
  off: '23:00',
  on: '07:00',
  days: [1, 2, 3, 4, 5],
  enabled: true,
  acknowledge_lockout: true,
};

describe('wifi schedule rules', () => {
  it('a new rule targets the first non-guest AP, every day, 23:00 to 07:00', () => {
    const aps = [
      { ...ap, guest: true, section: 'guest', ssid: 'Guest' },
      { ...ap, section: 'default_radio1', ssid: 'Home' },
    ];
    expect(emptyForm(aps)).toMatchObject({
      id: '',
      section: 'default_radio1',
      off: '23:00',
      on: '07:00',
    });
    expect(emptyForm(aps).days).toHaveLength(7);
  });

  it('refuses no days and equal times before the daemon does', () => {
    const form = formFrom(rule);
    expect(formProblem(form)).toBeUndefined();
    expect(formProblem({ ...form, days: [] })).toBe('Pick at least one day.');
    expect(formProblem({ ...form, on: '23:00' })).toBe('Off and on times must differ.');
  });

  it('adds a rule with sorted days and no acknowledgement unless consented', () => {
    const form = { ...emptyForm([]), section: 'x', days: [0, 6, 6] };
    expect(upsertRule([], form, false)).toEqual([
      { ...form, days: [0, 6], acknowledge_lockout: false },
    ]);
    expect(upsertRule([], form, true)[0]?.acknowledge_lockout).toBe(true);
  });

  it('an edit keeps its acknowledgement only while the AP stays the same', () => {
    expect(upsertRule([rule], { ...formFrom(rule), off: '22:00' }, false)).toEqual([
      { ...rule, off: '22:00' },
    ]);
    const moved = upsertRule([rule], { ...formFrom(rule), section: 'other' }, false);
    expect(moved).toHaveLength(1);
    expect(moved[0]?.acknowledge_lockout).toBe(false);
  });

  it('pauses, resumes and removes one rule', () => {
    const other = { ...rule, id: 'ffff0000', acknowledge_lockout: false };
    expect(setRuleEnabled([rule, other], other.id, false, false)[1]?.enabled).toBe(false);
    expect(setRuleEnabled([rule, other], other.id, true, true)[1]?.acknowledge_lockout).toBe(true);
    expect(setRuleEnabled([rule, other], other.id, true, true)[0]).toBe(rule);
    expect(removeRule([rule, other], rule.id)).toEqual([other]);
  });

  it('labels day sets as people say them, Monday first', () => {
    expect(daysLabel([0, 1, 2, 3, 4, 5, 6])).toBe('every day');
    expect(daysLabel([1, 2, 3, 4, 5])).toBe('Mon to Fri');
    expect(daysLabel([0, 6])).toBe('Sat and Sun');
    expect(daysLabel([0, 1, 3])).toBe('Mon, Wed, Sun');
  });

  it('shows the newest run of a rule on the router clock', () => {
    const runs = [
      { id: rule.id, at: 100, action: 'off', outcome: 'applied' },
      { id: rule.id, at: 300, action: 'on', outcome: 'unchanged' },
      { id: 'other', at: 500, action: 'off', outcome: 'failed' },
    ] as const;
    expect(lastRun([...runs], rule.id)?.at).toBe(300);
    expect(lastRun([...runs], 'none')).toBeUndefined();
    // 2026-11-16 22:30 UTC is Monday 23:30 in Berlin.
    expect(runTime(Date.UTC(2026, 10, 16, 22, 30) / 1000, 'Europe/Berlin')).toBe('Mon 23:30');
    expect(runTime(0, 'Not/AZone')).toMatch(/\d\d:\d\d/);
  });
});

describe('wifi schedule api', () => {
  it('loads rules, runs, APs and the router zone together', async () => {
    vi.mocked(getWiFiSchedule).mockResolvedValue(reply({ rules: [rule] }));
    vi.mocked(getWiFiScheduleStatus).mockResolvedValue(reply({ cron_running: true, runs: [] }));
    vi.mocked(getWiFiAccessPoints).mockResolvedValue(reply({ aps: [] }));
    vi.mocked(getSystemSettings).mockResolvedValue(reply({ zonename: 'Europe/Berlin' }));
    await expect(fetchSchedule()).resolves.toEqual({
      rules: [rule],
      runs: [],
      cronRunning: true,
      aps: [],
      zone: 'Europe/Berlin',
    });
  });

  it('works without status and settings; fails without rules', async () => {
    vi.mocked(getWiFiSchedule).mockResolvedValue(reply({ rules: [] }));
    vi.mocked(getWiFiScheduleStatus).mockResolvedValue(refused('x', 503));
    vi.mocked(getWiFiAccessPoints).mockResolvedValue(reply({ aps: [] }));
    vi.mocked(getSystemSettings).mockRejectedValue(new Error('offline'));
    await expect(fetchSchedule()).resolves.toMatchObject({ cronRunning: null, zone: undefined });
    vi.mocked(getWiFiSchedule).mockResolvedValue(refused('wifi_schedule_unavailable', 503));
    await expect(fetchSchedule()).rejects.toThrow('The WiFi schedule is unavailable.');
  });

  it('separates a lockout refusal from other errors and sends the set as given', async () => {
    vi.mocked(setWiFiSchedule).mockResolvedValueOnce(refused('wifi_lockout_risk'));
    await expect(saveSchedule([rule])).resolves.toEqual({ lockout: true });
    expect(vi.mocked(setWiFiSchedule).mock.calls[0]?.[0]?.body).toEqual({ rules: [rule] });
    vi.mocked(setWiFiSchedule).mockResolvedValueOnce(refused('wifi_ap_not_found', 404));
    await expect(saveSchedule([rule])).resolves.toEqual({
      error: scheduleError('wifi_ap_not_found'),
    });
    vi.mocked(setWiFiSchedule).mockResolvedValueOnce(reply({ rules: [rule] }));
    await expect(saveSchedule([rule])).resolves.toEqual({ rules: [rule] });
    vi.mocked(setWiFiSchedule).mockRejectedValueOnce(new Error('offline'));
    await expect(saveSchedule([rule])).resolves.toEqual({ error: 'Could not save the schedule.' });
  });
});

describe('wifi schedule lockout consent', () => {
  const next = (ack: boolean) => [{ ...rule, acknowledge_lockout: ack }];

  it('saves without asking when the daemon accepts', async () => {
    const ask = vi.fn();
    const send = vi.fn().mockResolvedValue({ rules: [rule] });
    await expect(saveWithConsent(next, ask, send)).resolves.toEqual({ rules: [rule] });
    expect(ask).not.toHaveBeenCalled();
    expect(send.mock.calls[0]?.[0]?.[0]?.acknowledge_lockout).toBe(false);
  });

  it('resends with the acknowledgement only after explicit consent', async () => {
    const send = vi
      .fn()
      .mockResolvedValueOnce({ lockout: true })
      .mockResolvedValueOnce({ rules: [rule] });
    await expect(saveWithConsent(next, async () => true, send)).resolves.toEqual({ rules: [rule] });
    expect(send.mock.calls.map((c) => c[0][0].acknowledge_lockout)).toEqual([false, true]);
  });

  it('a declined warning sends nothing more', async () => {
    const send = vi.fn().mockResolvedValue({ lockout: true });
    await expect(saveWithConsent(next, async () => false, send)).resolves.toEqual({
      cancelled: true,
    });
    expect(send).toHaveBeenCalledTimes(1);
  });
});
