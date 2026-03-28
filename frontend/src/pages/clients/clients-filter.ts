import type { Client } from '@shared/index';

export function filterClientsBySearch(clients: Client[], search: string): Client[] {
  if (!search) return clients;
  const q = search.toLowerCase();
  return clients.filter(
    (c) =>
      c.ip_address.includes(q) ||
      c.mac_address.toLowerCase().includes(q) ||
      (c.hostname ?? '').toLowerCase().includes(q) ||
      (c.alias ?? '').toLowerCase().includes(q),
  );
}
