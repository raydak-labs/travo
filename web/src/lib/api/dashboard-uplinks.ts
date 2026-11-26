import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getDashboardUplinks } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchDashboardUplinks(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getDashboardUplinks({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.dashboard_uplinks_unavailable(), response?.status);
  return data;
}
