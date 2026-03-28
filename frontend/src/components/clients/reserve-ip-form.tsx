import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAddDHCPReservation } from '@/hooks/use-network';
import {
  dhcpReservationFormSchema,
  type DhcpReservationFormValues,
} from '@/lib/schemas/network-forms';

type ReserveIpFormProps = {
  initial: DhcpReservationFormValues;
  onCancel: () => void;
};

export function ReserveIpForm({ initial, onCancel }: ReserveIpFormProps) {
  const addReservation = useAddDHCPReservation();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<DhcpReservationFormValues>({
    resolver: zodResolver(dhcpReservationFormSchema),
    defaultValues: initial,
    mode: 'onChange',
  });

  return (
    <form
      onSubmit={handleSubmit((data) =>
        addReservation.mutate(
          {
            mac: data.mac.trim(),
            ip: data.ip.trim(),
            name: data.name.trim(),
          },
          { onSuccess: onCancel },
        ),
      )}
      className="mt-4 rounded-md border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950"
      noValidate
    >
      <p className="mb-2 text-sm font-medium text-blue-900 dark:text-blue-200">
        Reserve IP for {initial.mac}
      </p>
      <div className="flex flex-wrap items-end gap-2">
        <div className="flex flex-col gap-0.5">
          <Input
            placeholder="Hostname"
            className="h-8 w-40 text-sm"
            aria-invalid={errors.name ? 'true' : undefined}
            {...register('name')}
          />
          {errors.name ? (
            <span className="text-xs text-red-500" role="alert">
              {errors.name.message}
            </span>
          ) : null}
        </div>
        <div className="flex flex-col gap-0.5">
          <Input
            placeholder="IP address"
            className="h-8 w-36 text-sm font-mono"
            aria-invalid={errors.ip ? 'true' : undefined}
            {...register('ip')}
          />
          {errors.ip ? (
            <span className="text-xs text-red-500" role="alert">
              {errors.ip.message}
            </span>
          ) : null}
        </div>
        <input type="hidden" {...register('mac')} />
        <Button type="submit" size="sm" disabled={addReservation.isPending}>
          {addReservation.isPending ? 'Saving…' : 'Save'}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
