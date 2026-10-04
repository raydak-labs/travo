import type { FieldErrors, UseFormHandleSubmit, UseFormRegister } from 'react-hook-form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { DhcpReservationFormValues } from '@/lib/schemas/network-forms';

type DhcpReservationAddFormProps = {
  register: UseFormRegister<DhcpReservationFormValues>;
  handleSubmit: UseFormHandleSubmit<DhcpReservationFormValues>;
  onValidAdd: (data: DhcpReservationFormValues) => void;
  errors: FieldErrors<DhcpReservationFormValues>;
  addPending: boolean;
};

export function DhcpReservationAddForm({
  register,
  handleSubmit,
  onValidAdd,
  errors,
  addPending,
}: DhcpReservationAddFormProps) {
  return (
    <form
      onSubmit={handleSubmit(onValidAdd)}
      className="grid grid-cols-1 items-end gap-2 sm:grid-cols-[1fr_1fr_1fr_auto]"
      noValidate
    >
      <div className="space-y-1">
        <Label htmlFor="dhcp-name">Name</Label>
        <Input
          id="dhcp-name"
          placeholder="laptop"
          aria-invalid={errors.name ? 'true' : undefined}
          aria-describedby={errors.name ? 'dhcp-name-err' : undefined}
          {...register('name')}
        />
        {errors.name ? (
          <p id="dhcp-name-err" className="text-xs text-red-600 dark:text-red-400" role="alert">
            {errors.name.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1">
        <Label htmlFor="dhcp-mac">MAC Address</Label>
        <Input
          id="dhcp-mac"
          placeholder="AA:BB:CC:DD:EE:FF"
          className="font-mono"
          aria-invalid={errors.mac ? 'true' : undefined}
          aria-describedby={errors.mac ? 'dhcp-mac-err' : undefined}
          {...register('mac')}
        />
        {errors.mac ? (
          <p id="dhcp-mac-err" className="text-xs text-red-600 dark:text-red-400" role="alert">
            {errors.mac.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1">
        <Label htmlFor="dhcp-ip">IP Address</Label>
        <Input
          id="dhcp-ip"
          placeholder="192.168.8.50"
          className="font-mono"
          aria-invalid={errors.ip ? 'true' : undefined}
          aria-describedby={errors.ip ? 'dhcp-ip-err' : undefined}
          {...register('ip')}
        />
        {errors.ip ? (
          <p id="dhcp-ip-err" className="text-xs text-red-600 dark:text-red-400" role="alert">
            {errors.ip.message}
          </p>
        ) : null}
      </div>
      <Button type="submit" disabled={addPending}>
        {addPending ? 'Adding…' : 'Add'}
      </Button>
    </form>
  );
}
