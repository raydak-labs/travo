import type { FieldErrors, UseFormHandleSubmit, UseFormRegister } from 'react-hook-form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import type { LedScheduleFormValues } from '@/lib/schemas/system-forms';

type LedScheduleFormProps = {
  register: UseFormRegister<LedScheduleFormValues>;
  handleSubmit: UseFormHandleSubmit<LedScheduleFormValues>;
  errors: FieldErrors<LedScheduleFormValues>;
  scheduleEnabled: boolean;
  onSave: (data: LedScheduleFormValues) => void;
  savePending: boolean;
  /** Server still has schedule while form toggle is off — show remove control */
  serverScheduleActive: boolean;
  onRemoveSchedule: () => void;
  removePending: boolean;
};

export function LedScheduleForm({
  register,
  handleSubmit,
  errors,
  scheduleEnabled,
  onSave,
  savePending,
  serverScheduleActive,
  onRemoveSchedule,
  removePending,
}: LedScheduleFormProps) {
  return (
    <form onSubmit={handleSubmit(onSave)} className="space-y-3 rounded-lg border p-3" noValidate>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-gray-900 dark:text-white">LED Schedule</p>
          <p className="text-xs text-gray-500">Automatically turn LEDs on/off at set times</p>
        </div>
        <Switch id="led-schedule" label="Enable schedule" {...register('enabled')} />
      </div>
      {scheduleEnabled && (
        <div className="space-y-2">
          <div className="flex items-center gap-3">
            <label htmlFor="led-on-time" className="w-16 text-xs text-gray-600 dark:text-gray-400">
              LEDs On
            </label>
            <Input
              id="led-on-time"
              type="time"
              className="w-32"
              aria-invalid={!!errors.on_time}
              aria-describedby={errors.on_time ? 'led-on-err' : undefined}
              {...register('on_time')}
            />
          </div>
          {errors.on_time ? (
            <p id="led-on-err" className="text-xs text-red-500" role="alert">
              {errors.on_time.message}
            </p>
          ) : null}
          <div className="flex items-center gap-3">
            <label htmlFor="led-off-time" className="w-16 text-xs text-gray-600 dark:text-gray-400">
              LEDs Off
            </label>
            <Input
              id="led-off-time"
              type="time"
              className="w-32"
              aria-invalid={!!errors.off_time}
              aria-describedby={errors.off_time ? 'led-off-err' : undefined}
              {...register('off_time')}
            />
          </div>
          {errors.off_time ? (
            <p id="led-off-err" className="text-xs text-red-500" role="alert">
              {errors.off_time.message}
            </p>
          ) : null}
          <Button size="sm" type="submit" disabled={savePending}>
            {savePending ? 'Saving...' : 'Save Schedule'}
          </Button>
        </div>
      )}
      {!scheduleEnabled && serverScheduleActive && (
        <Button
          size="sm"
          type="button"
          variant="outline"
          disabled={removePending}
          onClick={onRemoveSchedule}
        >
          Remove Schedule
        </Button>
      )}
    </form>
  );
}
