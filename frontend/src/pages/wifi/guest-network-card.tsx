import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ShieldCheck } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useGuestWifi, useSetGuestWifi } from '@/hooks/use-wifi';
import type { GuestWifiConfig } from '@shared/index';
import { guestWifiFormSchema, type GuestWifiFormValues } from '@/lib/schemas/wifi-forms';
import { normalizeGuestEncryption } from './guest-wifi-encryption';
import { GuestWifiEnabledFields } from './guest-wifi-enabled-fields';

export function GuestNetworkCard() {
  const { data: guestWifi, isLoading: guestLoading } = useGuestWifi();
  const setGuestWifi = useSetGuestWifi();

  const {
    register,
    handleSubmit,
    control,
    reset,
    watch,
    setValue,
    formState: { errors },
  } = useForm<GuestWifiFormValues>({
    resolver: zodResolver(guestWifiFormSchema),
    defaultValues: {
      enabled: false,
      ssid: '',
      encryption: 'psk2',
      key: '',
    },
    mode: 'onChange',
  });

  const enabled = watch('enabled');
  const encryption = watch('encryption');

  useEffect(() => {
    if (guestWifi) {
      reset({
        enabled: guestWifi.enabled,
        ssid: guestWifi.ssid,
        encryption: normalizeGuestEncryption(guestWifi.encryption),
        key: guestWifi.key,
      });
    }
  }, [guestWifi, reset]);

  const onSave = (data: GuestWifiFormValues) => {
    const payload: GuestWifiConfig = {
      enabled: data.enabled,
      ssid: data.enabled ? data.ssid.trim() : data.ssid,
      encryption: data.encryption,
      key: data.encryption === 'none' ? '' : data.key,
    };
    setGuestWifi.mutate(payload);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Guest Network</CardTitle>
        <ShieldCheck className="h-4 w-4 text-gray-400" />
      </CardHeader>
      <CardContent>
        {guestLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : (
          <form onSubmit={handleSubmit(onSave)} className="space-y-4" noValidate>
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  Enable Guest WiFi
                </span>
                <p className="text-xs text-gray-500">
                  Separate network (192.168.2.0/24) with client isolation
                </p>
              </div>
              <Switch id="guest-enabled" label="Enabled" {...register('enabled')} />
            </div>
            {enabled && (
              <GuestWifiEnabledFields
                register={register}
                control={control}
                errors={errors}
                encryption={encryption}
                setValue={setValue}
              />
            )}
            <Button type="submit" size="sm" disabled={setGuestWifi.isPending}>
              {setGuestWifi.isPending ? 'Saving...' : 'Save'}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
