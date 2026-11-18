import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type AdGuardStatus,
  getAdGuard,
  installAdGuard,
  setAdGuardDns,
  setAdGuardService,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export async function fetchAdGuard(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getAdGuard({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.adguard_unavailable(), response?.status);
  }
  return data;
}

export type AdGuardAction = 'install' | 'service' | 'dns';

/** Maps the API's stable error codes to a message; never parses error text. */
export function adguardError(code: string | undefined): string {
  switch (code) {
    case 'adguard_offline':
      return m.adguard_err_offline();
    case 'adguard_no_space':
      return m.adguard_err_no_space();
    case 'adguard_not_answering':
      return m.adguard_err_not_answering();
    case 'adguard_busy':
    case 'dns_busy':
      return m.adguard_err_busy();
    case 'adguard_config_unknown':
      return m.adguard_err_config();
    default:
      return m.adguard_err_failed();
  }
}

/** Runs one change; resolves to the new status, or an error message. */
export async function changeAdGuard(
  action: AdGuardAction,
  enabled: boolean,
  signal?: AbortSignal,
): Promise<{ status: AdGuardStatus } | { error: string }> {
  try {
    const options = { signal, body: { enabled } };
    const response =
      action === 'install'
        ? await installAdGuard({ signal })
        : action === 'service'
          ? await setAdGuardService(options)
          : await setAdGuardDns(options);
    if (response.data) return { status: response.data };
    return { error: adguardError((response.error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: m.adguard_err_failed() };
  }
}

/**
 * The gated proxy is the router's own address on port 3001 (ADR 0006), or 3443 for a page
 * loaded over HTTPS: its Secure session cookie never reaches the plain proxy (ADR 0016).
 */
export function adguardUrl(location: Pick<Location, 'protocol' | 'hostname'>): string {
  const host = location.hostname.includes(':') ? `[${location.hostname}]` : location.hostname;
  const port = location.protocol === 'https:' ? 3443 : 3001;
  return `${location.protocol}//${host}:${port}/`;
}
