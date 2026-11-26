import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getSystemAlerts } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchAlerts(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getSystemAlerts({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.alerts_unavailable(), response?.status);
  return data;
}
