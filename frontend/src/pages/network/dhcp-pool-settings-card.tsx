import { useEffect } from 'react';
import { useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Settings } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { QueryCard } from '@/components/ui/query-card';
import { useDHCPConfig, useSetDHCPConfig } from '@/hooks/use-network';
import {
  dhcpPoolFormSchema,
  normalizeDhcpLeaseTime,
  type DhcpPoolFormValues,
} from '@/lib/schemas/network-forms';
import { DhcpPoolFormFields } from './dhcp-pool-form-fields';

export function DhcpPoolSettingsCard() {
  const {
    data: dhcpConfig,
    isLoading: dhcpLoading,
    isError: dhcpFailed,
    error: dhcpError,
    refetch: refetchDhcp,
  } = useDHCPConfig();
  const setDHCP = useSetDHCPConfig();

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors: dhcpErrors },
  } = useForm<DhcpPoolFormValues>({
    resolver: zodResolver(dhcpPoolFormSchema) as Resolver<DhcpPoolFormValues>,
    defaultValues: { start: 100, limit: 150, lease_time: '12h' },
    mode: 'onChange',
  });

  useEffect(() => {
    if (dhcpConfig) {
      reset({
        start: dhcpConfig.start,
        limit: dhcpConfig.limit,
        lease_time: normalizeDhcpLeaseTime(dhcpConfig.lease_time),
      });
    }
  }, [dhcpConfig, reset]);

  const onSaveDhcp = (data: DhcpPoolFormValues) => {
    setDHCP.mutate({
      start: data.start,
      limit: data.limit,
      lease_time: data.lease_time,
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>DHCP Configuration</CardTitle>
        <Settings className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        {/* The form is only editable once the pool has been read: a failed GET
            with hard-coded defaults would let a Save overwrite the real pool. */}
        <QueryCard
          isLoading={dhcpLoading}
          isError={dhcpFailed}
          error={dhcpError}
          onRetry={() => void refetchDhcp()}
        >
          {dhcpConfig ? (
            <form onSubmit={handleSubmit(onSaveDhcp)} className="space-y-4" noValidate>
              <DhcpPoolFormFields register={register} control={control} errors={dhcpErrors} />
              <Button type="submit" disabled={setDHCP.isPending} size="sm">
                {setDHCP.isPending ? 'Saving…' : 'Save DHCP Settings'}
              </Button>
            </form>
          ) : null}
        </QueryCard>
      </CardContent>
    </Card>
  );
}
