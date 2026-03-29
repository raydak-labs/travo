import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Lightbulb } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import {
  useLEDStatus,
  useSetLEDStealth,
  useLEDSchedule,
  useSetLEDSchedule,
} from '@/hooks/use-system';
import { ledScheduleFormSchema, type LedScheduleFormValues } from '@/lib/schemas/system-forms';
import { LedStealthStatusPanel } from './led-stealth-status-panel';
import { LedScheduleForm } from './led-schedule-form';

export function LEDControlCard() {
  const { data: ledStatus } = useLEDStatus();
  const setLEDStealthMutation = useSetLEDStealth();
  const { data: ledSchedule } = useLEDSchedule();
  const setLEDScheduleMutation = useSetLEDSchedule();

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<LedScheduleFormValues>({
    resolver: zodResolver(ledScheduleFormSchema),
    defaultValues: {
      enabled: false,
      on_time: '07:00',
      off_time: '22:00',
    },
  });

  const scheduleEnabled = watch('enabled');

  useEffect(() => {
    if (ledSchedule) {
      reset({
        enabled: ledSchedule.enabled,
        on_time: ledSchedule.on_time || '07:00',
        off_time: ledSchedule.off_time || '22:00',
      });
    }
  }, [ledSchedule, reset]);

  const onSaveSchedule = (data: LedScheduleFormValues) => {
    setLEDScheduleMutation.mutate({
      enabled: data.enabled,
      on_time: data.on_time,
      off_time: data.off_time,
    });
  };

  const onRemoveSchedule = () => {
    setLEDScheduleMutation.mutate(
      { enabled: false, on_time: '', off_time: '' },
      {
        onSuccess: () =>
          reset({
            enabled: false,
            on_time: '07:00',
            off_time: '22:00',
          }),
      },
    );
  };

  if (!ledStatus || ledStatus.led_count === 0) return null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">LED Control</CardTitle>
        <Lightbulb className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        <LedStealthStatusPanel
          ledStatus={ledStatus}
          stealthPending={setLEDStealthMutation.isPending}
          onToggleStealth={() =>
            setLEDStealthMutation.mutate({ stealth_mode: !ledStatus.stealth_mode })
          }
        />
        <LedScheduleForm
          register={register}
          handleSubmit={handleSubmit}
          errors={errors}
          scheduleEnabled={scheduleEnabled}
          onSave={onSaveSchedule}
          savePending={setLEDScheduleMutation.isPending}
          serverScheduleActive={!!ledSchedule?.enabled}
          onRemoveSchedule={onRemoveSchedule}
          removePending={setLEDScheduleMutation.isPending}
        />
      </CardContent>
    </Card>
  );
}
