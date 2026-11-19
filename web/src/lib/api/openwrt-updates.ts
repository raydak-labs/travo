import { m } from '../paraglide/messages.js';
import { client } from './client';
import { checkOpenWrtUpdates, getOpenWrtUpdates, type OpenWrtCheck } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchOpenWrtUpdates(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<OpenWrtCheck> {
  const { data, response } = await getOpenWrtUpdates({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.osupdate_unavailable(), response?.status);
  return data;
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function openWrtCheckError(code: string | undefined): string {
  return code === 'update_check_offline' ? m.osupdate_err_offline() : m.osupdate_err_failed();
}

export async function runOpenWrtCheck(
  refreshLists: boolean,
): Promise<{ result: OpenWrtCheck } | { error: string }> {
  try {
    const { data, error } = await checkOpenWrtUpdates({ body: { refresh_lists: refreshLists } });
    if (data) return { result: data };
    return { error: openWrtCheckError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: openWrtCheckError(undefined) };
  }
}

/**
 * LuCI attended sysupgrade on the same host. install.sh moves LuCI to :8080 (HTTP) and
 * :8443 (HTTPS) next to Travo (ADR 0007); the flash keeps installed packages there.
 */
export function attendedSysupgradeURL(location: Pick<Location, 'protocol' | 'hostname'>): string {
  const https = location.protocol === 'https:';
  const host = location.hostname.includes(':') ? `[${location.hostname}]` : location.hostname;
  return `${https ? 'https' : 'http'}://${host}:${https ? 8443 : 8080}/cgi-bin/luci/admin/system/attendedsysupgrade`;
}
