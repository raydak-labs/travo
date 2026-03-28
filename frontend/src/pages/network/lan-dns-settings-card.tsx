import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Globe } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useDNSConfig, useSetDNSConfig } from '@/hooks/use-network';
import { dnsConfigFormSchema, type DnsConfigFormValues } from '@/lib/schemas/network-forms';
import { LanDnsPresetButtons } from './lan-dns-preset-buttons';
import { LanDnsServerFields } from './lan-dns-server-fields';

export function LanDnsSettingsCard() {
  const { data: dnsConfig, isLoading: dnsLoading } = useDNSConfig();
  const setDNS = useSetDNSConfig();

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors: dnsErrors },
  } = useForm<DnsConfigFormValues>({
    resolver: zodResolver(dnsConfigFormSchema),
    defaultValues: {
      use_custom_dns: false,
      server1: '',
      server2: '',
    },
    mode: 'onChange',
  });

  const useCustom = watch('use_custom_dns');

  useEffect(() => {
    if (dnsConfig) {
      reset({
        use_custom_dns: dnsConfig.use_custom_dns,
        server1: dnsConfig.servers?.[0] || '',
        server2: dnsConfig.servers?.[1] || '',
      });
    }
  }, [dnsConfig, reset]);

  const onSaveDns = (data: DnsConfigFormValues) => {
    const servers = [data.server1, data.server2].map((s) => s.trim()).filter(Boolean);
    setDNS.mutate({ use_custom_dns: data.use_custom_dns, servers });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">DNS Configuration</CardTitle>
        <Globe className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {dnsLoading ? (
          <Skeleton className="h-4 w-1/2" />
        ) : (
          <form onSubmit={handleSubmit(onSaveDns)} className="space-y-4" noValidate>
            <div className="flex items-center gap-2">
              <Switch {...register('use_custom_dns')} />
              <span className="text-sm">Use custom DNS servers</span>
            </div>
            {useCustom && (
              <>
                <LanDnsPresetButtons
                  onPick={(primary, secondary) => {
                    setValue('server1', primary, { shouldValidate: true });
                    setValue('server2', secondary, { shouldValidate: true });
                  }}
                />
                <LanDnsServerFields register={register} errors={dnsErrors} />
              </>
            )}
            <Button type="submit" size="sm" disabled={setDNS.isPending}>
              {setDNS.isPending ? 'Saving…' : 'Save DNS Settings'}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
