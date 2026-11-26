import type { LanClient, Reservation } from './api/clients';
import { m } from './paraglide/messages.js';
import type { StatusKind } from './ui/status';

export type ClientEditForm = { alias: string; reserved: boolean; ip: string; name: string };

export type ClientEditStep =
  | { kind: 'alias'; alias: string }
  | { kind: 'reserve'; ip: string; name: string }
  | { kind: 'unreserve' };

export type ClientEdit = { mac: string; steps: ClientEditStep[] };

/** What the edit dialog must send: only the parts that changed, alias first. */
export function planClientEdit(
  client: Pick<LanClient, 'mac' | 'alias'>,
  reservation: Pick<Reservation, 'ip' | 'name'> | undefined,
  form: ClientEditForm,
): ClientEdit {
  const steps: ClientEditStep[] = [];
  const alias = form.alias.trim();
  if (alias !== (client.alias ?? '')) steps.push({ kind: 'alias', alias });
  const ip = form.ip.trim();
  const name = form.name.trim();
  if (form.reserved) {
    if (!reservation || reservation.ip !== ip || (reservation.name ?? '') !== name) {
      steps.push({ kind: 'reserve', ip, name });
    }
  } else if (reservation) {
    steps.push({ kind: 'unreserve' });
  }
  return { mac: client.mac, steps };
}

/** Badge for how the router sees the client; `lan` never claims Ethernet (br-lan carries both). */
export function connectionStatus(c: Pick<LanClient, 'connection' | 'signal'>): {
  kind: StatusKind;
  label: string;
} {
  switch (c.connection) {
    case 'wifi':
      return {
        kind: 'info',
        label:
          c.signal === undefined
            ? m.clients_conn_wifi()
            : `${m.clients_conn_wifi()} · ${m.clients_signal({ signal: c.signal })}`,
      };
    case 'lan':
      return { kind: 'info', label: m.clients_conn_lan() };
    default:
      return { kind: 'neutral', label: m.clients_conn_offline() };
  }
}
