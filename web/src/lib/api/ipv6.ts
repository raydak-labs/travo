import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getIpv6Settings, type IPv6View } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { IPv6View };

/** Maps a refused PUT /network/ipv6 to its message; never parses error text. */
export function ipv6Refusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'ipv6_foreign_config':
      return { error: m.ipv6_err_foreign() };
    case 'ipv6_vpn_leak':
      return { error: m.ipv6_err_vpn_leak() };
    case 'ipv6_no_change':
      return { message: m.ipv6_no_change() };
    case 'uci_busy':
      return { error: m.dhcp_err_busy() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.ipv6_failed() };
  }
}

export async function fetchIPv6(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<IPv6View> {
  const { data, error, response } = await getIpv6Settings({ signal, client: requestClient });
  if (!data) {
    const code = (error as { code?: string } | undefined)?.code;
    throw new APIRequestError(
      code === 'ipv6_foreign_config' ? m.ipv6_err_foreign() : m.ipv6_unavailable(),
      response?.status,
    );
  }
  return data;
}
