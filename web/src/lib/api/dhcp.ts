import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  addDnsName,
  type DhcpSettings,
  type DhcpView,
  type DnsName,
  type DnsNamesOutputBody,
  deleteDnsName,
  getDhcpSettings,
  listDnsNames,
  setDhcpSettings,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { DhcpSettings, DhcpView, DnsName };

/** Mirrors the daemon's lease rule: <n>m (n ≥ 2), <n>h, <n>d, or infinite. */
export function validLeaseTime(value: string): boolean {
  const v = value.trim();
  if (v === 'infinite') return true;
  const match = /^([1-9][0-9]{0,5})([mhd])$/.exec(v);
  return match !== null && !(match[2] === 'm' && match[1] === '1');
}

/** Whether a save turns the server off: the dialog then warns that devices lose their address. */
export function turnsOff(
  before: Pick<DhcpSettings, 'enabled'>,
  after: Pick<DhcpSettings, 'enabled'>,
): boolean {
  return before.enabled && !after.enabled;
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function dhcpError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.dhcp_err_invalid();
    case 'dhcp_foreign_config':
      return m.dhcp_err_foreign();
    case 'uci_busy':
      return m.dhcp_err_busy();
    case 'dns_name_exists':
      return m.dns_names_err_exists();
    case 'dns_names_full':
      return m.dns_names_err_full();
    case 'dns_name_foreign':
      return m.dns_names_err_foreign();
    default:
      return m.dhcp_err_failed();
  }
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function fetchDHCP(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DhcpView> {
  const { data, error, response } = await getDhcpSettings({ signal, client: requestClient });
  if (!data) throw new APIRequestError(dhcpError(code(error)), response?.status);
  return data;
}

export async function saveDHCP(
  settings: DhcpSettings,
  requestClient: Client = client,
): Promise<{ view: DhcpView } | { error: string }> {
  try {
    const { data, error } = await setDhcpSettings({ body: settings, client: requestClient });
    return data ? { view: data } : { error: dhcpError(code(error)) };
  } catch {
    return { error: dhcpError(undefined) };
  }
}

export async function fetchDNSNames(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DnsNamesOutputBody> {
  const { data, response } = await listDnsNames({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.dns_names_unavailable(), response?.status);
  return data;
}

export async function addDNSName(
  name: string,
  ip: string,
  requestClient: Client = client,
): Promise<{ entry: DnsName } | { error: string }> {
  try {
    const { data, error } = await addDnsName({
      body: { name: name.trim(), ip: ip.trim() },
      client: requestClient,
    });
    return data ? { entry: data } : { error: dhcpError(code(error)) };
  } catch {
    return { error: dhcpError(undefined) };
  }
}

/** Removing an entry that is already gone counts as done. */
export async function removeDNSName(
  id: string,
  requestClient: Client = client,
): Promise<{ ok: true } | { error: string }> {
  try {
    const { data, error } = await deleteDnsName({ path: { id }, client: requestClient });
    if (data || code(error) === 'not_found') return { ok: true };
    return { error: dhcpError(code(error)) };
  } catch {
    return { error: dhcpError(undefined) };
  }
}
