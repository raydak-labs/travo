import { ClientsConnectedClientsCard } from '@/pages/clients/clients-connected-clients-card';
import { ClientsDhcpReservationsCard } from '@/components/clients/clients-dhcp-reservations-card';

export function ClientsPage() {
  return (
    <div className="space-y-4 p-4">
      <ClientsConnectedClientsCard />
      <ClientsDhcpReservationsCard />
    </div>
  );
}
