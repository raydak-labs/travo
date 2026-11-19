import { m } from '../paraglide/messages.js';
import { client } from './client';
import { listPortForwards, type PortForward, type PortForwardRequest } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { PortForward, PortForwardRequest };
export type ForwardProto = PortForwardRequest['proto'];

/** The add form's editable state. */
export type ForwardForm = {
  name: string;
  proto: ForwardProto;
  externalPort: string;
  targetIP: string;
  targetPort: string;
};

export function emptyForm(): ForwardForm {
  return { name: '', proto: 'tcp', externalPort: '', targetIP: '', targetPort: '' };
}

/** Builds the strict POST body: blanks trimmed, empty optional fields omitted. */
export function requestFrom(form: ForwardForm): PortForwardRequest {
  const body: PortForwardRequest = {
    proto: form.proto,
    external_port: form.externalPort.trim(),
    target_ip: form.targetIP.trim(),
  };
  const name = form.name.trim();
  const targetPort = form.targetPort.trim();
  if (name) body.name = name;
  if (targetPort) body.target_port = targetPort;
  return body;
}

/** Maps a refused add or remove to its message; never parses error text. */
export function forwardRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'invalid_request':
      return { error: m.port_forwards_err_invalid() };
    case 'port_forward_in_use':
      return { error: m.port_forwards_err_in_use() };
    case 'port_forwards_full':
      return { error: m.port_forwards_err_full() };
    case 'port_forward_no_zone':
      return { error: m.port_forwards_err_no_zone() };
    case 'port_forward_foreign':
      return { error: m.port_forwards_err_foreign() };
    case 'not_found':
      return { message: m.port_forwards_err_missing() };
    case 'uci_busy':
      return { error: m.dhcp_err_busy() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.port_forwards_failed() };
  }
}

/** The target as people read it: ip, then :port when it differs from the external one. */
export function targetLabel(f: Pick<PortForward, 'target_ip' | 'target_port'>): string {
  const ip = f.target_ip || m.port_forwards_router();
  return f.target_port ? `${ip}:${f.target_port}` : ip;
}

export async function fetchPortForwards(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<PortForward[]> {
  const { data, response } = await listPortForwards({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.port_forwards_unavailable(), response?.status);
  return data.forwards;
}
