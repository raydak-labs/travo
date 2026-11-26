import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getDashboardConnectivity } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchConnectivity(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getDashboardConnectivity({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.dashboard_connectivity_unavailable(), response?.status);
  }
  return data;
}
