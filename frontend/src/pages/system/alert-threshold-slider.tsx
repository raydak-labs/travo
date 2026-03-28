import type { UseFormRegister } from 'react-hook-form';
import type { AlertThresholdsFormValues } from '@/lib/schemas/system-forms';

type AlertThresholdSliderProps = {
  label: string;
  fieldName: keyof AlertThresholdsFormValues;
  value: number;
  register: UseFormRegister<AlertThresholdsFormValues>;
};

export function AlertThresholdSlider({
  label,
  fieldName,
  value,
  register,
}: AlertThresholdSliderProps) {
  return (
    <div className="space-y-2">
      <div className="flex justify-between text-sm">
        <span className="text-gray-700 dark:text-gray-300">{label}</span>
        <span className="font-medium">{value}%</span>
      </div>
      <input
        type="range"
        min={50}
        max={99}
        step={1}
        className="w-full accent-blue-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        {...register(fieldName, { valueAsNumber: true })}
      />
    </div>
  );
}
