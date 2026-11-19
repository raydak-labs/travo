import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getWanSettings, type WanRequest, type WanView } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { WanRequest, WanView };
export type WanProto = WanView['proto'];

/** The form's editable state; the password is write-only and starts empty. */
export type WanForm = {
  proto: WanProto;
  ipaddr: string;
  gateway: string;
  dns: string;
  username: string;
  password: string;
};

export function formFrom(view: WanView): WanForm {
  return {
    proto: view.proto,
    ipaddr: view.ipaddr,
    gateway: view.gateway,
    dns: view.dns.join(', '),
    username: view.username,
    password: '',
  };
}

/** Splits the DNS field on commas and whitespace, blanks dropped. */
export function parseDNS(text: string): string[] {
  return text
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter((s) => s !== '');
}

/** Builds the strict PUT body: only the chosen protocol's fields, an empty password omitted. */
export function requestFrom(form: WanForm): WanRequest {
  switch (form.proto) {
    case 'static':
      return {
        proto: 'static',
        ipaddr: form.ipaddr.trim(),
        gateway: form.gateway.trim(),
        dns: parseDNS(form.dns),
      };
    case 'pppoe':
      return form.password
        ? { proto: 'pppoe', username: form.username.trim(), password: form.password }
        : { proto: 'pppoe', username: form.username.trim() };
    default:
      return { proto: 'dhcp' };
  }
}

/** Maps a refused PUT /network/wan to its message; never parses error text. */
export function wanRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'invalid_request':
      return { error: m.wan_err_invalid() };
    case 'wan_overlaps_lan':
      return { error: m.wan_err_overlaps_lan() };
    case 'wan_foreign_config':
      return { error: m.wan_err_foreign() };
    case 'wan_no_change':
      return { message: m.wan_no_change() };
    case 'uci_busy':
      return { error: m.dhcp_err_busy() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.wan_failed() };
  }
}

export async function fetchWAN(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<WanView> {
  const { data, error, response } = await getWanSettings({ signal, client: requestClient });
  if (!data) {
    const code = (error as { code?: string } | undefined)?.code;
    throw new APIRequestError(
      code === 'wan_foreign_config' ? m.wan_err_foreign() : m.wan_unavailable(),
      response?.status,
    );
  }
  return data;
}
