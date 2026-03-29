import {
  Controller,
  type Control,
  type FieldErrors,
  type UseFormRegister,
  type UseFormSetValue,
} from 'react-hook-form';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { DdnsFormValues } from '@/lib/schemas/network-forms';

type DdnsEnabledFieldsProps = {
  control: Control<DdnsFormValues>;
  register: UseFormRegister<DdnsFormValues>;
  errors: FieldErrors<DdnsFormValues>;
  service: string;
  setValue: UseFormSetValue<DdnsFormValues>;
};

export function DdnsEnabledFields({
  control,
  register,
  errors,
  service,
  setValue,
}: DdnsEnabledFieldsProps) {
  return (
    <>
      <div className="space-y-1">
        <label className="text-xs text-gray-500">Provider</label>
        <Controller
          control={control}
          name="service"
          render={({ field }) => (
            <Select
              value={field.value}
              onValueChange={(v) => {
                field.onChange(v);
                if (v !== 'custom') setValue('update_url', '');
              }}
            >
              <SelectTrigger aria-invalid={errors.service ? 'true' : undefined}>
                <SelectValue placeholder="Select a DDNS provider" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="duckdns.org">DuckDNS</SelectItem>
                <SelectItem value="no-ip.com">No-IP</SelectItem>
                <SelectItem value="cloudflare.com-v4">Cloudflare</SelectItem>
                <SelectItem value="freedns.afraid.org">FreeDNS</SelectItem>
                <SelectItem value="dynu.com">Dynu</SelectItem>
                <SelectItem value="desec.io">deSEC</SelectItem>
                <SelectItem value="custom">Custom (update URL)</SelectItem>
              </SelectContent>
            </Select>
          )}
        />
        {errors.service ? (
          <p className="text-xs text-red-500" role="alert">
            {errors.service.message}
          </p>
        ) : null}
      </div>
      {service === 'custom' && (
        <div className="space-y-1">
          <label className="text-xs text-gray-500 dark:text-gray-400">Update URL</label>
          <textarea
            rows={3}
            placeholder="https://example.com/update?hostname=[DOMAIN]&myip=[IP]"
            className={cn(
              'flex w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder:text-gray-500',
              errors.update_url && 'border-red-500',
            )}
            aria-invalid={errors.update_url ? 'true' : undefined}
            aria-describedby={errors.update_url ? 'ddns-url-err' : undefined}
            {...register('update_url')}
          />
          {errors.update_url ? (
            <p id="ddns-url-err" className="text-xs text-red-500" role="alert">
              {errors.update_url.message}
            </p>
          ) : null}
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Use ddns-scripts placeholders such as [IP], [DOMAIN], [USERNAME], [PASSWORD] as required
            by your provider.
          </p>
        </div>
      )}
      <div className="space-y-1">
        <label className="text-xs text-gray-500">Domain</label>
        <Input
          placeholder="myrouter.duckdns.org"
          aria-invalid={errors.domain ? 'true' : undefined}
          aria-describedby={errors.domain ? 'ddns-domain-err' : undefined}
          {...register('domain')}
        />
        {errors.domain ? (
          <p id="ddns-domain-err" className="text-xs text-red-500" role="alert">
            {errors.domain.message}
          </p>
        ) : null}
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1">
          <label className="text-xs text-gray-500">Username / Token</label>
          <Input placeholder="username or token" {...register('username')} />
        </div>
        <div className="space-y-1">
          <label className="text-xs text-gray-500">Password</label>
          <Input type="password" placeholder="password" {...register('password')} />
        </div>
      </div>
      <div className="space-y-1">
        <label className="text-xs text-gray-500">Lookup Host</label>
        <Input placeholder="myrouter.duckdns.org" {...register('lookup_host')} />
      </div>
    </>
  );
}
