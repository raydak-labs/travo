import { BookmarkPlus, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useDHCPReservations, useDeleteDHCPReservation } from '@/hooks/use-network';

export function ClientsDhcpReservationsCard() {
  const { data: reservations } = useDHCPReservations();
  const deleteReservation = useDeleteDHCPReservation();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Static IP Reservations</CardTitle>
        <BookmarkPlus className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {!reservations || reservations.length === 0 ? (
          <p className="text-sm text-gray-500">
            No static reservations. Use the bookmark icon next to a connected client to reserve its
            IP.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200 text-left dark:border-gray-700">
                  <th className="pb-2 font-medium text-gray-500">Hostname</th>
                  <th className="pb-2 font-medium text-gray-500">MAC</th>
                  <th className="pb-2 font-medium text-gray-500">IP</th>
                  <th className="pb-2 text-right font-medium text-gray-500">Actions</th>
                </tr>
              </thead>
              <tbody>
                {reservations.map((r) => (
                  <tr
                    key={r.section ?? r.mac}
                    className="border-b border-gray-100 last:border-0 dark:border-gray-800"
                  >
                    <td className="py-2 pr-4 text-gray-900 dark:text-white">{r.name}</td>
                    <td className="py-2 pr-4 font-mono text-xs text-gray-500">{r.mac}</td>
                    <td className="py-2 pr-4 text-gray-700 dark:text-gray-300">{r.ip}</td>
                    <td className="py-2 text-right">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-red-600"
                        title="Remove reservation"
                        onClick={() => r.section && deleteReservation.mutate(r.section)}
                        disabled={!r.section || deleteReservation.isPending}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
