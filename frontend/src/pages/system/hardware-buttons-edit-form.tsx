import type { Control, FieldArrayWithId, UseFormHandleSubmit } from 'react-hook-form';
import { Controller } from 'react-hook-form';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { HardwareButtonsFormValues } from '@/lib/schemas/system-forms';
import { HARDWARE_BUTTON_ACTION_OPTIONS } from './hardware-button-actions';

type HardwareButtonsEditFormProps = {
  fields: FieldArrayWithId<HardwareButtonsFormValues, 'buttons', 'id'>[];
  control: Control<HardwareButtonsFormValues>;
  handleSubmit: UseFormHandleSubmit<HardwareButtonsFormValues>;
  onValidSubmit: (data: HardwareButtonsFormValues) => void;
  onCancel: () => void;
  savePending: boolean;
};

export function HardwareButtonsEditForm({
  fields,
  control,
  handleSubmit,
  onValidSubmit,
  onCancel,
  savePending,
}: HardwareButtonsEditFormProps) {
  return (
    <form onSubmit={handleSubmit(onValidSubmit)} className="space-y-3" noValidate>
      {fields.map((field, index) => (
        <div key={field.id} className="flex items-center justify-between gap-4">
          <span className="font-mono text-sm capitalize">{field.name}</span>
          <Controller
            control={control}
            name={`buttons.${index}.action`}
            render={({ field: f }) => (
              <Select value={f.value} onValueChange={f.onChange}>
                <SelectTrigger className="w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {HARDWARE_BUTTON_ACTION_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
      ))}

      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" type="submit" disabled={savePending}>
          {savePending ? 'Saving…' : 'Save Button Actions'}
        </Button>

        <Button type="button" size="sm" variant="outline" onClick={onCancel} disabled={savePending}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
