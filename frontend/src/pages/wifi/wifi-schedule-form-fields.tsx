import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import type { WifiScheduleFormValues } from '@/lib/schemas/wifi-forms';

type WiFiScheduleFormFieldsProps = {
  enabled: boolean;
  register: UseFormRegister<WifiScheduleFormValues>;
  errors: FieldErrors<WifiScheduleFormValues>;
  switchDisabled: boolean;
};

export function WiFiScheduleFormFields({
  enabled,
  register,
  errors,
  switchDisabled,
}: WiFiScheduleFormFieldsProps) {
  return (
    <>
      <div className="flex items-center justify-between">
        <div className="space-y-0.5">
          <p className="text-sm">Enable schedule</p>
          <p className="text-xs text-gray-500">
            Automatically turn WiFi on and off at set times (cron-based)
          </p>
        </div>
        <Switch
          id="wifi-schedule-toggle"
          label="Enable schedule"
          disabled={switchDisabled}
          {...register('enabled')}
        />
      </div>

      {enabled && (
        <div className="space-y-3 rounded-md border p-3">
          <div className="flex flex-wrap items-center gap-3">
            <label
              className="w-16 shrink-0 text-sm text-gray-600 dark:text-gray-400"
              htmlFor="wifi-on-time"
            >
              On at
            </label>
            <div className="flex flex-col gap-0.5">
              <Input
                id="wifi-on-time"
                type="time"
                className="max-w-[140px]"
                aria-invalid={errors.on_time ? 'true' : undefined}
                aria-describedby={errors.on_time ? 'wifi-on-err' : undefined}
                {...register('on_time')}
              />
              {errors.on_time ? (
                <span id="wifi-on-err" className="text-xs text-red-500" role="alert">
                  {errors.on_time.message}
                </span>
              ) : null}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <label
              className="w-16 shrink-0 text-sm text-gray-600 dark:text-gray-400"
              htmlFor="wifi-off-time"
            >
              Off at
            </label>
            <div className="flex flex-col gap-0.5">
              <Input
                id="wifi-off-time"
                type="time"
                className="max-w-[140px]"
                aria-invalid={errors.off_time ? 'true' : undefined}
                aria-describedby={errors.off_time ? 'wifi-off-err' : undefined}
                {...register('off_time')}
              />
              {errors.off_time ? (
                <span id="wifi-off-err" className="text-xs text-red-500" role="alert">
                  {errors.off_time.message}
                </span>
              ) : null}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
