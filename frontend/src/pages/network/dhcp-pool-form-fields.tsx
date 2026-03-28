import { Controller, type Control, type FieldErrors, type UseFormRegister } from 'react-hook-form';
import { Input } from '@/components/ui/input';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  DHCP_LEASE_TIME_OPTIONS,
  formatDhcpLeaseTimeHumanLabel,
  type DhcpPoolFormValues,
} from '@/lib/schemas/network-forms';

type DhcpPoolFormFieldsProps = {
  register: UseFormRegister<DhcpPoolFormValues>;
  control: Control<DhcpPoolFormValues>;
  errors: FieldErrors<DhcpPoolFormValues>;
};

export function DhcpPoolFormFields({ register, control, errors }: DhcpPoolFormFieldsProps) {
  return (
    <>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1">
          <label className="flex items-center gap-1 text-xs text-gray-500">
            Start Offset
            <InfoTooltip text="First IP offset assigned to clients. Offset 100 on 192.168.1.x means the first assigned address is 192.168.1.100." />
          </label>
          <Input
            type="number"
            min={2}
            max={254}
            aria-invalid={errors.start ? 'true' : undefined}
            aria-describedby={errors.start ? 'dhcp-start-err' : undefined}
            {...register('start', { valueAsNumber: true })}
          />
          {errors.start ? (
            <p id="dhcp-start-err" className="text-xs text-red-500" role="alert">
              {errors.start.message}
            </p>
          ) : null}
        </div>
        <div className="space-y-1">
          <label className="flex items-center gap-1 text-xs text-gray-500">
            Pool Size
            <InfoTooltip text="Maximum number of clients that can receive an IP address. E.g., 50 means up to 50 devices can connect." />
          </label>
          <Input
            type="number"
            min={1}
            max={253}
            aria-invalid={errors.limit ? 'true' : undefined}
            aria-describedby={errors.limit ? 'dhcp-limit-err' : undefined}
            {...register('limit', { valueAsNumber: true })}
          />
          {errors.limit ? (
            <p id="dhcp-limit-err" className="text-xs text-red-500" role="alert">
              {errors.limit.message}
            </p>
          ) : null}
        </div>
      </div>
      <div className="space-y-1">
        <label className="flex items-center gap-1 text-xs text-gray-500">
          Lease Time
          <InfoTooltip text="How long a DHCP lease is valid before renewal. Shorter times reclaim IPs faster; longer times reduce DHCP traffic." />
        </label>
        <Controller
          name="lease_time"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger aria-invalid={errors.lease_time ? 'true' : undefined}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DHCP_LEASE_TIME_OPTIONS.map((opt) => (
                  <SelectItem key={opt} value={opt}>
                    {formatDhcpLeaseTimeHumanLabel(opt)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
      </div>
    </>
  );
}
