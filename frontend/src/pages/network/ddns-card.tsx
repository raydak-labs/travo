import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { RefreshCw } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useDDNSConfig, useDDNSStatus, useSetDDNSConfig } from '@/hooks/use-network';
import { ddnsFormSchema, type DdnsFormValues } from '@/lib/schemas/network-forms';
import { DdnsStatusPanel } from './ddns-status-panel';
import { DdnsEnabledFields } from './ddns-enabled-fields';

export function DdnsCard() {
  const { data: ddnsConfig, isLoading: ddnsConfigLoading } = useDDNSConfig();
  const { data: ddnsStatus } = useDDNSStatus();
  const setDDNS = useSetDDNSConfig();

  const {
    register,
    control,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors },
  } = useForm<DdnsFormValues>({
    resolver: zodResolver(ddnsFormSchema),
    defaultValues: {
      enabled: false,
      service: '',
      domain: '',
      username: '',
      password: '',
      lookup_host: '',
      update_url: '',
    },
    mode: 'onChange',
  });

  const enabled = watch('enabled');
  const service = watch('service');

  useEffect(() => {
    if (ddnsConfig) {
      reset({
        enabled: ddnsConfig.enabled,
        service: ddnsConfig.service,
        domain: ddnsConfig.domain,
        username: ddnsConfig.username,
        password: ddnsConfig.password,
        lookup_host: ddnsConfig.lookup_host,
        update_url: ddnsConfig.update_url ?? '',
      });
    }
  }, [ddnsConfig, reset]);

  const onSubmit = (data: DdnsFormValues) => {
    setDDNS.mutate({
      enabled: data.enabled,
      service: data.service,
      domain: data.domain.trim(),
      username: data.username,
      password: data.password,
      lookup_host: data.lookup_host.trim(),
      update_url: data.service === 'custom' ? data.update_url.trim() : '',
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Dynamic DNS (DDNS)</CardTitle>
        <RefreshCw className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {ddnsConfigLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
            <DdnsStatusPanel status={ddnsStatus} />
            <div className="flex items-center gap-2">
              <Switch {...register('enabled')} />
              <span className="text-sm">Enable Dynamic DNS</span>
            </div>
            {enabled && (
              <DdnsEnabledFields
                control={control}
                register={register}
                errors={errors}
                service={service}
                setValue={setValue}
              />
            )}
            <Button type="submit" size="sm" disabled={setDDNS.isPending}>
              {setDDNS.isPending ? 'Saving…' : 'Save DDNS Settings'}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
