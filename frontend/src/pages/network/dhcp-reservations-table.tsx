import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { DHCPReservation } from '@shared/index';

type DhcpReservationsTableProps = {
  reservations: DHCPReservation[];
  onDeleteSection: (section: string) => void;
  deletePending: boolean;
};

export function DhcpReservationsTable({
  reservations,
  onDeleteSection,
  deletePending,
}: DhcpReservationsTableProps) {
  if (reservations.length === 0) return null;

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-gray-500">
            <th className="pb-2 font-medium">Name</th>
            <th className="pb-2 font-medium">MAC Address</th>
            <th className="pb-2 font-medium">IP Address</th>
            <th className="w-16 pb-2 font-medium"></th>
          </tr>
        </thead>
        <tbody>
          {reservations.map((reservation) => (
            <tr
              key={reservation.section ?? `${reservation.mac}-${reservation.ip}`}
              className="border-b last:border-0"
            >
              <td className="py-2 text-gray-900 dark:text-white">{reservation.name}</td>
              <td className="py-2 font-mono text-gray-500">{reservation.mac}</td>
              <td className="py-2 font-mono text-gray-900 dark:text-white">{reservation.ip}</td>
              <td className="py-2 text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  type="button"
                  onClick={() => reservation.section && onDeleteSection(reservation.section)}
                  disabled={deletePending}
                >
                  <Trash2 className="h-4 w-4 text-red-500" />
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
