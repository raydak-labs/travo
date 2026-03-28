import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useMACAddresses, useSetMAC, useRandomizeMAC } from '@/hooks/use-wifi';
import { macCloneFormSchema, type MacCloneFormValues } from '@/lib/schemas/wifi-forms';
import { MacAddressCloneBlock } from './mac-address-clone-block';
import { generateRandomMac } from './mac-address-utils';

export function MACAddressCard() {
  const { data: macAddresses, isLoading: macLoading } = useMACAddresses();
  const setMAC = useSetMAC();
  const randomizeMAC = useRandomizeMAC();

  const {
    register,
    handleSubmit,
    setValue,
    reset,
    formState: { errors },
  } = useForm<MacCloneFormValues>({
    resolver: zodResolver(macCloneFormSchema),
    defaultValues: { custom_mac: '' },
    mode: 'onChange',
  });

  useEffect(() => {
    if (macAddresses && macAddresses.length > 0) {
      reset({ custom_mac: macAddresses[0].custom_mac || '' });
    }
  }, [macAddresses, reset]);

  const onApply = (data: MacCloneFormValues) => {
    setMAC.mutate(data.custom_mac);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">MAC Address Cloning</CardTitle>
      </CardHeader>
      <CardContent>
        {macLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : !macAddresses || macAddresses.length === 0 ? (
          <p className="text-sm text-gray-500">No STA interface detected</p>
        ) : (
          <form onSubmit={handleSubmit(onApply)} className="space-y-4" noValidate>
            {macAddresses.map((mac) => (
              <MacAddressCloneBlock
                key={mac.interface}
                mac={mac}
                register={register}
                errors={errors}
                onRandomLocal={() =>
                  setValue('custom_mac', generateRandomMac(), { shouldValidate: true })
                }
                onRandomizeApply={() => randomizeMAC.mutate()}
                onResetDefault={() => {
                  setValue('custom_mac', '', { shouldValidate: true });
                  setMAC.mutate('');
                }}
                setMacPending={setMAC.isPending}
                randomizePending={randomizeMAC.isPending}
              />
            ))}
          </form>
        )}
      </CardContent>
    </Card>
  );
}
