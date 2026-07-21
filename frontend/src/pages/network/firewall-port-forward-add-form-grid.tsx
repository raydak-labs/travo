import { Controller, type Control, type FieldErrors, type UseFormRegister } from 'react-hook-form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { PortForwardFormValues } from '@/lib/schemas/network-forms';

type FirewallPortForwardAddFormGridProps = {
  register: UseFormRegister<PortForwardFormValues>;
  control: Control<PortForwardFormValues>;
  errors: FieldErrors<PortForwardFormValues>;
  isPending: boolean;
};

export function FirewallPortForwardAddFormGrid({
  register,
  control,
  errors,
  isPending,
}: FirewallPortForwardAddFormGridProps) {
  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-[1fr_auto_1fr_1fr_1fr_auto]">
      <div className="space-y-1">
        <Label>Name</Label>
        <Input
          placeholder="my-rule"
          aria-invalid={errors.name ? 'true' : undefined}
          aria-describedby={errors.name ? 'pf-name-err' : undefined}
          {...register('name')}
        />
        {errors.name ? (
          <p id="pf-name-err" className="text-xs text-red-500" role="alert">
            {errors.name.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1">
        <Label>Protocol</Label>
        <Controller
          name="protocol"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger className="w-full lg:w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="tcp">TCP</SelectItem>
                <SelectItem value="udp">UDP</SelectItem>
                <SelectItem value="tcp udp">Both</SelectItem>
              </SelectContent>
            </Select>
          )}
        />
      </div>
      <div className="space-y-1">
        <Label>External Port</Label>
        <Input
          placeholder="8080"
          aria-invalid={errors.src_dport ? 'true' : undefined}
          aria-describedby={errors.src_dport ? 'pf-ext-err' : undefined}
          {...register('src_dport')}
        />
        {errors.src_dport ? (
          <p id="pf-ext-err" className="text-xs text-red-500" role="alert">
            {errors.src_dport.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1">
        <Label>Internal IP</Label>
        <Input
          placeholder="192.168.8.10"
          aria-invalid={errors.dest_ip ? 'true' : undefined}
          aria-describedby={errors.dest_ip ? 'pf-ip-err' : undefined}
          {...register('dest_ip')}
        />
        {errors.dest_ip ? (
          <p id="pf-ip-err" className="text-xs text-red-500" role="alert">
            {errors.dest_ip.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1">
        <Label>Internal Port</Label>
        <Input
          placeholder="80"
          aria-invalid={errors.dest_port ? 'true' : undefined}
          aria-describedby={errors.dest_port ? 'pf-int-err' : undefined}
          {...register('dest_port')}
        />
        {errors.dest_port ? (
          <p id="pf-int-err" className="text-xs text-red-500" role="alert">
            {errors.dest_port.message}
          </p>
        ) : null}
      </div>
      <Button type="submit" className="self-end" disabled={isPending}>
        {isPending ? 'Adding…' : 'Add'}
      </Button>
    </div>
  );
}
