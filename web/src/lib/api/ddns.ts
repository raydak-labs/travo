import { m } from '../paraglide/messages.js';
import { client } from './client';
import { type DdnsRequest, type DdnsView, getDdns, installDdns, setDdns } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { DdnsRequest, DdnsView };

/** Maps the API's stable error codes to a message; never parses error text. */
export function ddnsError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.ddns_err_invalid();
    case 'ddns_not_installed':
      return m.ddns_err_not_installed();
    case 'ddns_foreign_config':
      return m.ddns_err_foreign();
    case 'ddns_no_change':
      return m.ddns_no_change();
    case 'ddns_busy':
      return m.ddns_err_busy();
    case 'ddns_offline':
      return m.ddns_err_offline();
    case 'ddns_no_space':
      return m.ddns_err_no_space();
    case 'uci_busy':
      return m.dhcp_err_busy();
    case 'transaction_pending':
      return m.vpn_tunnel_busy();
    default:
      return m.ddns_failed();
  }
}

/** The strict PUT body; an empty password keeps the stored one and is omitted. */
export function ddnsRequest(form: DdnsRequest): DdnsRequest {
  const body: DdnsRequest = {
    enabled: form.enabled,
    service: form.service,
    domain: form.domain.trim(),
  };
  if (form.username?.trim()) body.username = form.username.trim();
  if (form.password) body.password = form.password;
  return body;
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function fetchDDNS(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DdnsView> {
  const { data, error, response } = await getDdns({ signal, client: requestClient });
  if (!data) throw new APIRequestError(ddnsError(code(error)), response?.status);
  return data;
}

export async function saveDDNS(
  form: DdnsRequest,
  requestClient: Client = client,
): Promise<{ view: DdnsView } | { error: string }> {
  try {
    const { data, error } = await setDdns({ body: ddnsRequest(form), client: requestClient });
    return data ? { view: data } : { error: ddnsError(code(error)) };
  } catch {
    return { error: ddnsError(undefined) };
  }
}

export async function installDDNS(
  requestClient: Client = client,
): Promise<{ view: DdnsView } | { error: string }> {
  try {
    const { data, error } = await installDdns({ client: requestClient });
    return data ? { view: data } : { error: ddnsError(code(error)) };
  } catch {
    return { error: ddnsError(undefined) };
  }
}
