import { useState } from 'react';
import { Users } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { ReserveIpForm } from '@/components/clients/reserve-ip-form';
import { useClients, useBlockedClients, useDHCPReservations } from '@/hooks/use-network';
import type { Client } from '@shared/index';
import { ClientsConnectedTable } from '@/pages/clients/clients-connected-table';
import { filterClientsBySearch } from '@/pages/clients/clients-filter';
import { ClientsSearchBar } from '@/pages/clients/clients-search-bar';

export function ClientsConnectedClientsCard() {
  const { data: clients, isLoading: clientsLoading } = useClients();
  const { data: blockedMacs } = useBlockedClients();
  const { data: reservations } = useDHCPReservations();

  const [search, setSearch] = useState('');
  const [reserveForm, setReserveForm] = useState<{ mac: string; ip: string; name: string } | null>(
    null,
  );

  const blockedSet = new Set((blockedMacs ?? []).map((m) => m.toUpperCase()));
  const reservedMacs = new Set((reservations ?? []).map((r) => r.mac.toUpperCase()));

  const filtered = filterClientsBySearch(clients ?? [], search);

  const handleReserveIP = (client: Client) => {
    setReserveForm({
      mac: client.mac_address,
      ip: client.ip_address,
      name: client.alias || client.hostname || '',
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">
          Connected Clients
          {clients && (
            <span className="ml-2 text-xs font-normal text-gray-500">
              ({clients.length} device{clients.length !== 1 ? 's' : ''})
            </span>
          )}
        </CardTitle>
        <Users className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        <ClientsSearchBar value={search} onChange={setSearch} />

        <ClientsConnectedTable
          clientsLoading={clientsLoading}
          filtered={filtered}
          hasSearch={Boolean(search)}
          blockedSet={blockedSet}
          reservedMacs={reservedMacs}
          onReserveIP={handleReserveIP}
        />

        {reserveForm && (
          <ReserveIpForm
            key={reserveForm.mac}
            initial={{
              mac: reserveForm.mac,
              ip: reserveForm.ip,
              name: reserveForm.name,
            }}
            onCancel={() => setReserveForm(null)}
          />
        )}
      </CardContent>
    </Card>
  );
}
