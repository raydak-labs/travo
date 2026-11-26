import type { ClientEdit } from '../clients';
import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  deleteReservation,
  kickClient,
  type Client as LanClient,
  listClients,
  listReservations,
  type Reservation,
  reserveClient,
  setClientAlias,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { LanClient, Reservation };

export type ClientList = { clients: LanClient[]; partial: boolean };

export async function fetchClients(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<ClientList> {
  const { data, response } = await listClients({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.clients_unavailable(), response?.status);
  return data;
}

export async function fetchReservations(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<Reservation[]> {
  const { data, response } = await listReservations({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.clients_unavailable(), response?.status);
  return data.reservations;
}

/** Alias first, then the DHCP host name, then the MAC: the name people recognise. */
export function clientName(c: Pick<LanClient, 'alias' | 'hostname' | 'mac'>): string {
  return c.alias || c.hostname || c.mac;
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function clientError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.clients_err_invalid();
    case 'client_is_caller':
      return m.clients_err_caller();
    case 'client_not_associated':
      return m.clients_err_not_associated();
    case 'reservation_ip_taken':
      return m.clients_err_ip_taken();
    case 'reservation_shared':
      return m.clients_err_shared();
    case 'reservation_not_found':
      return m.clients_err_reservation_missing();
    case 'reservation_no_lan':
      return m.clients_err_no_lan();
    case 'client_no_change':
      return m.clients_err_no_change();
    case 'transaction_pending':
      return m.clients_err_busy();
    case 'uci_busy':
      return m.clients_err_uci_busy();
    default:
      return m.clients_err_failed();
  }
}

/** A refused block as the guarded toggle expects it: "no change" is information, not an error. */
export function blockRefusal(code: string | undefined): { message?: string; error?: string } {
  return code === 'client_no_change'
    ? { message: clientError(code) }
    : { error: clientError(code) };
}

type Outcome = { ok: true } | { error: string };

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

async function outcome(call: () => Promise<{ data?: unknown; error?: unknown }>): Promise<Outcome> {
  try {
    const { data, error } = await call();
    return data ? { ok: true } : { error: clientError(code(error)) };
  } catch {
    return { error: clientError(undefined) };
  }
}

export function saveAlias(mac: string, alias: string): Promise<Outcome> {
  return outcome(() => setClientAlias({ path: { mac }, body: { alias: alias.trim() } }));
}

export function reserve(mac: string, ip: string, name: string): Promise<Outcome> {
  const body = name.trim() ? { mac, ip: ip.trim(), name: name.trim() } : { mac, ip: ip.trim() };
  return outcome(() => reserveClient({ body }));
}

export function unreserve(mac: string): Promise<Outcome> {
  return outcome(() => deleteReservation({ path: { mac } }));
}

export function kick(mac: string): Promise<Outcome> {
  return outcome(() => kickClient({ path: { mac } }));
}

/** Runs the dialog's steps in order and stops at the first failure; resolves to its message. */
export async function runClientEdit(edit: ClientEdit): Promise<string | undefined> {
  for (const step of edit.steps) {
    const result =
      step.kind === 'alias'
        ? await saveAlias(edit.mac, step.alias)
        : step.kind === 'reserve'
          ? await reserve(edit.mac, step.ip, step.name)
          : await unreserve(edit.mac);
    if ('error' in result) return result.error;
  }
  return undefined;
}
