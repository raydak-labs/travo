import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/components/ui/input';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import type { DnsConfigFormValues } from '@/lib/schemas/network-forms';

type LanDnsServerFieldsProps = {
  register: UseFormRegister<DnsConfigFormValues>;
  errors: FieldErrors<DnsConfigFormValues>;
};

export function LanDnsServerFields({ register, errors }: LanDnsServerFieldsProps) {
  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-1">
        <label className="flex items-center gap-1 text-xs text-gray-500">
          Primary DNS
          <InfoTooltip text="DNS server that resolves domain names to IP addresses for all LAN clients. E.g., 8.8.8.8 (Google), 1.1.1.1 (Cloudflare), 9.9.9.9 (Quad9)." />
        </label>
        <Input
          placeholder="8.8.8.8"
          aria-invalid={errors.server1 ? 'true' : undefined}
          aria-describedby={errors.server1 ? 'lan-dns-s1-err' : undefined}
          {...register('server1')}
        />
        {errors.server1 ? (
          <p id="lan-dns-s1-err" className="text-xs text-red-500" role="alert">
            {errors.server1.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-1">
        <label className="text-xs text-gray-500">Secondary DNS</label>
        <Input placeholder="8.8.4.4" {...register('server2')} />
      </div>
    </div>
  );
}
