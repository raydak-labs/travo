import { Textarea } from '@/components/ui/textarea';
import {
  Controller,
  type Control,
  type FieldErrors,
  type UseFormRegister,
  type UseFormSetValue,
} from 'react-hook-form';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { FieldError } from '@/components/ui/field-error';
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
        <Label htmlFor="ddns-service">Provider</Label>
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
              <SelectTrigger id="ddns-service" aria-invalid={errors.service ? 'true' : undefined}>
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
        {errors.service ? <FieldError>{errors.service.message}</FieldError> : null}
      </div>
      {service === 'custom' && (
        <div className="space-y-1">
          <Label htmlFor="ddns-url">Update URL</Label>
          <Textarea
            id="ddns-url"
            placeholder="https://example.com/update?hostname=[DOMAIN]&myip=[IP]"
            aria-invalid={errors.update_url ? 'true' : undefined}
            aria-describedby={errors.update_url ? 'ddns-url-err' : undefined}
            {...register('update_url')}
          />
          {errors.update_url ? (
            <FieldError id="ddns-url-err">{errors.update_url.message}</FieldError>
          ) : null}
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Use ddns-scripts placeholders such as [IP], [DOMAIN], [USERNAME], [PASSWORD] as required
            by your provider.
          </p>
        </div>
      )}
      <div className="space-y-1">
        <Label htmlFor="ddns-domain">Domain</Label>
        <Input
          id="ddns-domain"
          placeholder="myrouter.duckdns.org"
          aria-invalid={errors.domain ? 'true' : undefined}
          aria-describedby={errors.domain ? 'ddns-domain-err' : undefined}
          {...register('domain')}
        />
        {errors.domain ? (
          <FieldError id="ddns-domain-err">{errors.domain.message}</FieldError>
        ) : null}
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1">
          <Label htmlFor="ddns-username">Username / Token</Label>
          <Input id="ddns-username" placeholder="username or token" {...register('username')} />
        </div>
        <div className="space-y-1">
          <Label htmlFor="ddns-password">Password</Label>
          <Input
            id="ddns-password"
            type="password"
            placeholder="password"
            {...register('password')}
          />
        </div>
      </div>
      <div className="space-y-1">
        <Label htmlFor="ddns-lookup-host">Lookup Host</Label>
        <Input
          id="ddns-lookup-host"
          placeholder="myrouter.duckdns.org"
          {...register('lookup_host')}
        />
      </div>
    </>
  );
}
