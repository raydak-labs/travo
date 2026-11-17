import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getVpnKillSwitch, getVpnStatus } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchVpnStatus(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getVpnStatus({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.vpn_status_unavailable(), response?.status);
  }
  return data;
}

export async function fetchKillSwitch(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getVpnKillSwitch({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.vpn_kill_unavailable(), response?.status);
  }
  return data;
}
