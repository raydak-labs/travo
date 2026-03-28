import { useState } from 'react';
import { HardDrive, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useDHCPReservations,
  useAddDHCPReservation,
  useDeleteDHCPReservation,
} from '@/hooks/use-network';

export function DhcpReservationsCard() {
  const { data: dhcpReservations, isLoading: dhcpReservationsLoading } = useDHCPReservations();
  const addDHCPReservation = useAddDHCPReservation();
  const deleteDHCPReservation = useDeleteDHCPReservation();
  const [newReservationName, setNewReservationName] = useState('');
  const [newReservationMAC, setNewReservationMAC] = useState('');
  const [newReservationIP, setNewReservationIP] = useState('');

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">DHCP Reservations</CardTitle>
        <HardDrive className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {dhcpReservationsLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : (
          <div className="space-y-4">
            {dhcpReservations && dhcpReservations.length > 0 && (
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
                    {dhcpReservations.map((reservation) => (
                      <tr key={reservation.section} className="border-b last:border-0">
                        <td className="py-2 text-gray-900 dark:text-white">{reservation.name}</td>
                        <td className="py-2 font-mono text-gray-500">{reservation.mac}</td>
                        <td className="py-2 font-mono text-gray-900 dark:text-white">
                          {reservation.ip}
                        </td>
                        <td className="py-2 text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() =>
                              reservation.section &&
                              deleteDHCPReservation.mutate(reservation.section)
                            }
                            disabled={deleteDHCPReservation.isPending}
                          >
                            <Trash2 className="h-4 w-4 text-red-500" />
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <div className="grid grid-cols-1 items-end gap-2 sm:grid-cols-[1fr_1fr_1fr_auto]">
              <div className="space-y-1">
                <label className="text-xs text-gray-500">Name</label>
                <Input
                  value={newReservationName}
                  onChange={(e) => setNewReservationName(e.target.value)}
                  placeholder="laptop"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs text-gray-500">MAC Address</label>
                <Input
                  value={newReservationMAC}
                  onChange={(e) => setNewReservationMAC(e.target.value)}
                  placeholder="AA:BB:CC:DD:EE:FF"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs text-gray-500">IP Address</label>
                <Input
                  value={newReservationIP}
                  onChange={(e) => setNewReservationIP(e.target.value)}
                  placeholder="192.168.8.50"
                />
              </div>
              <Button
                size="sm"
                onClick={() => {
                  if (newReservationName && newReservationMAC && newReservationIP) {
                    addDHCPReservation.mutate(
                      {
                        name: newReservationName,
                        mac: newReservationMAC,
                        ip: newReservationIP,
                      },
                      {
                        onSuccess: () => {
                          setNewReservationName('');
                          setNewReservationMAC('');
                          setNewReservationIP('');
                        },
                      },
                    );
                  }
                }}
                disabled={
                  addDHCPReservation.isPending ||
                  !newReservationName ||
                  !newReservationMAC ||
                  !newReservationIP
                }
              >
                {addDHCPReservation.isPending ? 'Adding…' : 'Add'}
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
