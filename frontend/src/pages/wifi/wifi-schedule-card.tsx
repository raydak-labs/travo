import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { useConnectionMethod } from '@/hooks/use-network';
import { useWiFiSchedule, useSetWiFiSchedule } from '@/hooks/use-wifi';
import { wifiScheduleFormSchema, type WifiScheduleFormValues } from '@/lib/schemas/wifi-forms';
import {
  describeScheduleLockout,
  formatClock,
  shouldWarnScheduleLockout,
  type ScheduleTimes,
} from '@/lib/wifi-schedule-lockout';
import { WiFiScheduleFormFields } from '@/pages/wifi/wifi-schedule-form-fields';

export function WiFiScheduleCard() {
  const { data: schedule, isLoading } = useWiFiSchedule();
  const setSchedule = useSetWiFiSchedule();
  const { data: connectionMethod } = useConnectionMethod();
  const [pendingSave, setPendingSave] = useState<WifiScheduleFormValues | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<WifiScheduleFormValues>({
    resolver: zodResolver(wifiScheduleFormSchema),
    defaultValues: {
      enabled: false,
      on_time: '08:00',
      off_time: '22:00',
    },
    mode: 'onChange',
  });

  const enabled = watch('enabled');

  const currentSchedule: ScheduleTimes | null = schedule
    ? {
        enabled: schedule.enabled,
        onTime: schedule.on_time || '08:00',
        offTime: schedule.off_time || '22:00',
      }
    : null;

  useEffect(() => {
    if (schedule) {
      reset({
        enabled: schedule.enabled,
        on_time: schedule.on_time || '08:00',
        off_time: schedule.off_time || '22:00',
      });
    }
  }, [schedule, reset]);

  const save = (data: WifiScheduleFormValues) => {
    setSchedule.mutate({
      enabled: data.enabled,
      on_time: data.on_time,
      off_time: data.off_time,
    });
  };

  const onSave = (data: WifiScheduleFormValues) => {
    const next: ScheduleTimes = {
      enabled: data.enabled,
      onTime: data.on_time,
      offTime: data.off_time,
    };
    const isWifiClient = connectionMethod?.method === 'wifi-client';
    // Enabling a schedule while this session reaches the router over WiFi is a
    // lockout with no way back until the On time, so it needs an explicit
    // confirm before the router is told anything.
    if (
      shouldWarnScheduleLockout({
        next,
        current: currentSchedule,
        connectionMethod: isWifiClient ? 'wifi-client' : undefined,
      })
    ) {
      setPendingSave(data);
      return;
    }
    save(data);
  };

  const pendingLockout = pendingSave
    ? describeScheduleLockout({
        enabled: pendingSave.enabled,
        onTime: pendingSave.on_time,
        offTime: pendingSave.off_time,
      })
    : null;

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>WiFi Schedule</CardTitle>
          <Clock className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>WiFi Schedule</CardTitle>
        <Clock className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onSave)} className="space-y-4" noValidate>
          <WiFiScheduleFormFields
            enabled={enabled}
            register={register}
            errors={errors}
            switchDisabled={setSchedule.isPending}
          />
          <Button type="submit" size="sm" disabled={setSchedule.isPending}>
            {setSchedule.isPending ? 'Saving…' : 'Save'}
          </Button>
        </form>
      </CardContent>

      <ConfirmDialog
        open={pendingSave !== null}
        onOpenChange={(open) => {
          if (!open) setPendingSave(null);
        }}
        title="This schedule can disconnect you"
        description={
          pendingLockout
            ? `You are reaching this router over WiFi. At ${formatClock(pendingLockout.offAt)} this router's WiFi turns off and you lose access to this page until ${formatClock(pendingLockout.onAt)}.`
            : 'You are reaching this router over WiFi. Turning WiFi off on a schedule disconnects you from this page.'
        }
        warningText="Connect an Ethernet cable to a LAN port if you need to keep managing the router at that time, or set the Off time to a moment you are not using it."
        confirmLabel="Save anyway"
        isPending={setSchedule.isPending}
        onConfirm={() => {
          if (!pendingSave) return;
          save(pendingSave);
          setPendingSave(null);
        }}
      />
    </Card>
  );
}
