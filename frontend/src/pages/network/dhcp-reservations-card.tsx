import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { HardDrive } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useDHCPReservations,
  useAddDHCPReservation,
  useDeleteDHCPReservation,
} from '@/hooks/use-network';
import {
  dhcpReservationFormSchema,
  type DhcpReservationFormValues,
} from '@/lib/schemas/network-forms';
import { DhcpReservationsTable } from './dhcp-reservations-table';
import { DhcpReservationAddForm } from './dhcp-reservation-add-form';

export function DhcpReservationsCard() {
  const { data: dhcpReservations, isLoading: dhcpReservationsLoading } = useDHCPReservations();
  const addDHCPReservation = useAddDHCPReservation();
  const deleteDHCPReservation = useDeleteDHCPReservation();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<DhcpReservationFormValues>({
    resolver: zodResolver(dhcpReservationFormSchema),
    defaultValues: { name: '', mac: '', ip: '' },
    mode: 'onChange',
  });

  const onAdd = (data: DhcpReservationFormValues) => {
    addDHCPReservation.mutate(
      {
        name: data.name.trim(),
        mac: data.mac.trim(),
        ip: data.ip.trim(),
      },
      {
        onSuccess: () => reset({ name: '', mac: '', ip: '' }),
      },
    );
  };

  const list = dhcpReservations ?? [];

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
            <DhcpReservationsTable
              reservations={list}
              onDeleteSection={(section) => deleteDHCPReservation.mutate(section)}
              deletePending={deleteDHCPReservation.isPending}
            />
            <DhcpReservationAddForm
              register={register}
              handleSubmit={handleSubmit}
              onValidAdd={onAdd}
              errors={errors}
              addPending={addDHCPReservation.isPending}
            />
          </div>
        )}
      </CardContent>
    </Card>
  );
}
