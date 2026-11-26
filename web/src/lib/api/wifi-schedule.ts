import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type ApConfig,
  getSystemSettings,
  getWiFiAccessPoints,
  getWiFiSchedule,
  getWiFiScheduleStatus,
  setWiFiSchedule,
  type WiFiScheduleRule,
  type WiFiScheduleRun,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { WiFiScheduleRule, WiFiScheduleRun };

/** What the schedule card shows: rules, their last toggles and the APs they can name. */
export type ScheduleView = {
  rules: WiFiScheduleRule[];
  runs: WiFiScheduleRun[];
  cronRunning: boolean | null;
  aps: ApConfig[];
  /** IANA zone of the router clock; undefined when the settings could not be read. */
  zone?: string;
};

export async function fetchSchedule(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<ScheduleView> {
  const options = { signal, client: requestClient };
  const [rules, status, aps, settings] = await Promise.all([
    getWiFiSchedule(options),
    getWiFiScheduleStatus(options),
    getWiFiAccessPoints(options),
    // The zone only labels the times; the card works without it.
    getSystemSettings(options).catch(() => undefined),
  ]);
  if (!rules.data) throw new APIRequestError(m.wifi_schedule_unavailable(), rules.response?.status);
  if (!aps.data) throw new APIRequestError(m.wifi_schedule_unavailable(), aps.response?.status);
  return {
    rules: rules.data.rules,
    runs: status.data?.runs ?? [],
    cronRunning: status.data ? status.data.cron_running : null,
    aps: aps.data.aps,
    zone: settings?.data?.zonename || undefined,
  };
}

/** The add/edit dialog's state; id is empty for a new rule. */
export type RuleForm = {
  id: string;
  section: string;
  off: string;
  on: string;
  days: number[];
  enabled: boolean;
};

export function emptyForm(aps: ApConfig[]): RuleForm {
  const first = aps.find((ap) => !ap.guest) ?? aps[0];
  return {
    id: '',
    section: first?.section ?? '',
    off: '23:00',
    on: '07:00',
    days: [0, 1, 2, 3, 4, 5, 6],
    enabled: true,
  };
}

export function formFrom(rule: WiFiScheduleRule): RuleForm {
  const { id, section, off, on, days, enabled } = rule;
  return { id, section, off, on, days: [...days], enabled };
}

/** Field problem the daemon would refuse with 400, or undefined. */
export function formProblem(form: RuleForm): string | undefined {
  if (form.days.length === 0) return m.wifi_schedule_err_days();
  if (form.off === form.on) return m.wifi_schedule_err_same();
  return undefined;
}

/**
 * Adds or replaces the form's rule. An edit keeps its acknowledgement while the AP stays the
 * same; `acknowledge` is set only after the operator consented in the lockout dialog.
 */
export function upsertRule(
  rules: WiFiScheduleRule[],
  form: RuleForm,
  acknowledge: boolean,
): WiFiScheduleRule[] {
  const old = form.id ? rules.find((r) => r.id === form.id) : undefined;
  const kept = old?.acknowledge_lockout === true && old.section === form.section;
  const rule: WiFiScheduleRule = {
    ...form,
    days: [...new Set(form.days)].sort((a, b) => a - b),
    acknowledge_lockout: acknowledge || kept,
  };
  return old ? rules.map((r) => (r.id === old.id ? rule : r)) : [...rules, rule];
}

/** Pauses or resumes one rule; `acknowledge` as in `upsertRule`. */
export function setRuleEnabled(
  rules: WiFiScheduleRule[],
  id: string,
  enabled: boolean,
  acknowledge: boolean,
): WiFiScheduleRule[] {
  return rules.map((r) =>
    r.id === id ? { ...r, enabled, acknowledge_lockout: r.acknowledge_lockout || acknowledge } : r,
  );
}

export function removeRule(rules: WiFiScheduleRule[], id: string): WiFiScheduleRule[] {
  return rules.filter((r) => r.id !== id);
}

const dayNames = () => [
  m.wifi_schedule_sun(),
  m.wifi_schedule_mon(),
  m.wifi_schedule_tue(),
  m.wifi_schedule_wed(),
  m.wifi_schedule_thu(),
  m.wifi_schedule_fri(),
  m.wifi_schedule_sat(),
];

/** Weekdays Monday first, as people read a week: 0 = Sunday comes last. */
export const weekOrder = [1, 2, 3, 4, 5, 6, 0];

export function dayName(day: number): string {
  return dayNames()[day] ?? String(day);
}

export function daysLabel(days: number[]): string {
  const set = new Set(days);
  if (set.size === 7) return m.wifi_schedule_every_day();
  if (set.size === 5 && !set.has(0) && !set.has(6)) return m.wifi_schedule_weekdays();
  if (set.size === 2 && set.has(0) && set.has(6)) return m.wifi_schedule_weekend();
  return weekOrder
    .filter((d) => set.has(d))
    .map(dayName)
    .join(', ');
}

/** The newest toggle of a rule, if any ran since the daemon started. */
export function lastRun(runs: WiFiScheduleRun[], id: string): WiFiScheduleRun | undefined {
  return runs.filter((r) => r.id === id).sort((a, b) => b.at - a.at)[0];
}

/** A toggle's time on the router clock (its zone), falling back to the browser's. */
export function runTime(at: number, zone: string | undefined): string {
  const options: Intl.DateTimeFormatOptions = {
    weekday: 'short',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  };
  const date = new Date(at * 1000);
  try {
    return date.toLocaleString('en', { ...options, timeZone: zone });
  } catch {
    return date.toLocaleString('en', options);
  }
}

export function outcomeLabel(outcome: WiFiScheduleRun['outcome']): string {
  switch (outcome) {
    case 'applied':
      return m.wifi_schedule_outcome_applied();
    case 'unchanged':
      return m.wifi_schedule_outcome_unchanged();
    case 'rolled_back':
      return m.wifi_schedule_outcome_rolled_back();
    case 'skipped_guard':
    case 'skipped_busy':
      return m.wifi_schedule_outcome_skipped();
    case 'skipped_lockout':
      return m.wifi_schedule_outcome_lockout();
    default:
      return m.wifi_schedule_outcome_failed();
  }
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function scheduleError(code: string | undefined): string {
  switch (code) {
    case 'wifi_schedule_invalid':
    case 'invalid_request':
      return m.wifi_schedule_err_invalid();
    case 'wifi_ap_not_found':
      return m.wifi_schedule_err_ap();
    default:
      return m.wifi_schedule_failed();
  }
}

export type SaveOutcome = { rules: WiFiScheduleRule[] } | { lockout: true } | { error: string };

/** Saves the whole rule set; a lockout refusal is its own outcome so the card can ask. */
export async function saveSchedule(
  rules: WiFiScheduleRule[],
  requestClient: Client = client,
): Promise<SaveOutcome> {
  try {
    const { data, error } = await setWiFiSchedule({ body: { rules }, client: requestClient });
    if (data) return { rules: data.rules };
    const code = (error as { code?: string } | undefined)?.code;
    return code === 'wifi_lockout_risk' ? { lockout: true } : { error: scheduleError(code) };
  } catch {
    return { error: scheduleError(undefined) };
  }
}

/**
 * Saves `next(false)`. A lockout refusal asks the operator and resends `next(true)` only
 * when `ask` resolves true (invariants §2, ADR 0027); a second refusal is an error.
 */
export async function saveWithConsent(
  next: (acknowledge: boolean) => WiFiScheduleRule[],
  ask: () => Promise<boolean>,
  send: (rules: WiFiScheduleRule[]) => Promise<SaveOutcome> = saveSchedule,
): Promise<{ rules: WiFiScheduleRule[] } | { error: string } | { cancelled: true }> {
  const first = await send(next(false));
  if (!('lockout' in first)) return first;
  if (!(await ask())) return { cancelled: true };
  const second = await send(next(true));
  return 'lockout' in second ? { error: scheduleError(undefined) } : second;
}
