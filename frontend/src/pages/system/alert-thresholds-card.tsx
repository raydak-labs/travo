import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Bell } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useAlertThresholds, useSetAlertThresholds } from '@/hooks/use-system';
import {
  alertThresholdsFormSchema,
  type AlertThresholdsFormValues,
} from '@/lib/schemas/system-forms';
import { AlertThresholdSlider } from '@/pages/system/alert-threshold-slider';

export function AlertThresholdsCard() {
  const { data, isLoading } = useAlertThresholds();
  const setThresholds = useSetAlertThresholds();

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<AlertThresholdsFormValues>({
    resolver: zodResolver(alertThresholdsFormSchema),
    defaultValues: {
      storage_percent: 90,
      cpu_percent: 90,
      memory_percent: 90,
    },
  });

  useEffect(() => {
    if (data) {
      reset({
        storage_percent: data.storage_percent,
        cpu_percent: data.cpu_percent,
        memory_percent: data.memory_percent,
      });
    }
  }, [data, reset]);

  const storage = watch('storage_percent');
  const cpu = watch('cpu_percent');
  const memory = watch('memory_percent');

  const onSubmit = (form: AlertThresholdsFormValues) => {
    setThresholds.mutate({
      storage_percent: form.storage_percent,
      cpu_percent: form.cpu_percent,
      memory_percent: form.memory_percent,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Bell className="h-5 w-5" />
          Alert Thresholds
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
          </div>
        ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-5" noValidate>
            <AlertThresholdSlider
              label="Storage %"
              fieldName="storage_percent"
              value={storage}
              register={register}
            />
            <AlertThresholdSlider
              label="CPU %"
              fieldName="cpu_percent"
              value={cpu}
              register={register}
            />
            <AlertThresholdSlider
              label="Memory %"
              fieldName="memory_percent"
              value={memory}
              register={register}
            />

            {errors.storage_percent?.message ? (
              <p className="text-sm text-red-500" role="alert">
                {errors.storage_percent.message}
              </p>
            ) : null}

            <Button type="submit" disabled={setThresholds.isPending} size="sm">
              {setThresholds.isPending ? 'Saving…' : 'Save Thresholds'}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
