import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getDashboardThroughput } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchThroughput(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getDashboardThroughput({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.dashboard_throughput_unavailable(), response?.status);
  }
  return data;
}
