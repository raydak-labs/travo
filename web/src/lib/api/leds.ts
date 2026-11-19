import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getLeds, type LedSchedule, type LedStatus, setLedSchedule, setLeds } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchLEDs(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<LedStatus> {
  const { data, response } = await getLeds({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.leds_unavailable(), response?.status);
  return data;
}

export function ledsError(code: string | undefined): string {
  switch (code) {
    case 'led_schedule_invalid':
      return m.leds_err_schedule();
    case 'uci_busy':
      return m.settings_err_busy();
    default:
      return m.leds_err_failed();
  }
}

/** Client check before saving; the daemon validates again. */
export function scheduleProblem(s: LedSchedule): string | undefined {
  const clock = /^([01][0-9]|2[0-3]):[0-5][0-9]$/;
  if (!clock.test(s.off_at) || !clock.test(s.on_at)) return m.leds_err_schedule();
  if (s.enabled && s.off_at === s.on_at) return m.leds_err_same_time();
  return undefined;
}

type Result = { status: LedStatus } | { error: string };

function result(data: LedStatus | undefined, error: unknown): Result {
  if (data) return { status: data };
  return { error: ledsError((error as { code?: string } | undefined)?.code) };
}

export async function setLEDsOff(off: boolean): Promise<Result> {
  try {
    const { data, error } = await setLeds({ body: { off } });
    return result(data, error);
  } catch {
    return { error: ledsError(undefined) };
  }
}

export async function saveLEDSchedule(schedule: LedSchedule): Promise<Result> {
  try {
    const { data, error } = await setLedSchedule({ body: schedule });
    return result(data, error);
  } catch {
    return { error: ledsError(undefined) };
  }
}
