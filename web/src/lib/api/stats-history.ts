import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getStatsHistory, resetStatsHistory } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type HistoryRange = 24 | 168;

export async function fetchStatsHistory(
  hours: HistoryRange,
  signal?: AbortSignal,
  requestClient: Client = client,
) {
  const { data, response } = await getStatsHistory({
    query: { hours },
    signal,
    client: requestClient,
  });
  if (!data) throw new APIRequestError(m.history_unavailable(), response?.status);
  return data;
}

/** Discards the history and its file; true when the daemon confirmed. */
export async function clearStatsHistory(requestClient: Client = client): Promise<boolean> {
  try {
    const { data } = await resetStatsHistory({ client: requestClient });
    return data !== undefined;
  } catch {
    return false;
  }
}
