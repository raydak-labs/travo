import { m } from '../paraglide/messages.js';
import type { StatusKind } from '../ui/status';
import { client } from './client';
import { type GetSystemLogData, getSystemLog, type LogEntry, type LogPage } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type LogLevel = NonNullable<NonNullable<GetSystemLogData['query']>['level']>;

/** Levels offered in the filter, most severe kept: "warn" shows warn, err, crit, alert, emerg. */
export const LOG_LEVELS: LogLevel[] = ['err', 'warn', 'notice', 'info', 'debug'];

export type LogQuery = { lines: number; level?: LogLevel; filter: string };

/** Only the parameters that narrow the query; an empty filter is not sent. */
export function logQuery(q: LogQuery): NonNullable<GetSystemLogData['query']> {
  const filter = q.filter.trim().slice(0, 128);
  return {
    lines: Math.min(Math.max(Math.trunc(q.lines) || 200, 1), 1000),
    ...(q.level ? { level: q.level } : {}),
    ...(filter ? { filter } : {}),
  };
}

export async function fetchSystemLog(
  q: LogQuery,
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<LogPage> {
  const { data, response } = await getSystemLog({
    query: logQuery(q),
    signal,
    client: requestClient,
  });
  if (!data) throw new APIRequestError(m.logs_unavailable(), response?.status);
  return data;
}

/** Status colour of a level; unknown levels are neutral. */
export function levelKind(level: LogEntry['level']): StatusKind {
  switch (level) {
    case 'emerg':
    case 'alert':
    case 'crit':
    case 'err':
      return 'danger';
    case 'warn':
      return 'warn';
    case 'notice':
      return 'info';
    default:
      return 'neutral';
  }
}

/** `2026-11-17T21:58:39` → `11-17 21:58:39`: the year is noise in a RAM log. */
export function shortTime(time: string): string {
  const match = /^\d{4}-(\d\d-\d\d)T(\d\d:\d\d:\d\d)$/.exec(time);
  return match ? `${match[1]} ${match[2]}` : time;
}
