import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useWiFiSchedule, useSetWiFiSchedule } from '@/hooks/use-wifi';
import { wifiScheduleFormSchema, type WifiScheduleFormValues } from '@/lib/schemas/wifi-forms';
import { WiFiScheduleFormFields } from '@/pages/wifi/wifi-schedule-form-fields';

export function WiFiScheduleCard() {
  const { data: schedule, isLoading } = useWiFiSchedule();
  const setSchedule = useSetWiFiSchedule();

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

  useEffect(() => {
    if (schedule) {
      reset({
        enabled: schedule.enabled,
        on_time: schedule.on_time || '08:00',
        off_time: schedule.off_time || '22:00',
      });
    }
  }, [schedule, reset]);

  const onSave = (data: WifiScheduleFormValues) => {
    setSchedule.mutate({
      enabled: data.enabled,
      on_time: data.on_time,
      off_time: data.off_time,
    });
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">WiFi Schedule</CardTitle>
          <Clock className="h-4 w-4 text-gray-400" />
        </CardHeader>
        <CardContent>
          <div className="h-16 animate-pulse rounded bg-gray-100 dark:bg-gray-800" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">WiFi Schedule</CardTitle>
        <Clock className="h-4 w-4 text-gray-400" />
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
    </Card>
  );
}
