import type {
  FieldErrors,
  UseFormRegister,
  UseFieldArrayReturn,
  UseFormReturn,
} from 'react-hook-form';
import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { NtpConfigFormValues, NtpServerDraftFormValues } from '@/lib/schemas/system-forms';

type NtpConfigServerFieldsProps = {
  register: UseFormRegister<NtpConfigFormValues>;
  errors: FieldErrors<NtpConfigFormValues>;
  fields: UseFieldArrayReturn<NtpConfigFormValues, 'servers', 'id'>['fields'];
  append: UseFieldArrayReturn<NtpConfigFormValues, 'servers', 'id'>['append'];
  remove: UseFieldArrayReturn<NtpConfigFormValues, 'servers', 'id'>['remove'];
  addServerForm: UseFormReturn<NtpServerDraftFormValues>;
};

export function NtpConfigServerFields({
  register,
  errors,
  fields,
  append,
  remove,
  addServerForm,
}: NtpConfigServerFieldsProps) {
  return (
    <div className="space-y-2">
      <Label>NTP Servers</Label>
      {fields.map((field, index) => (
        <div key={field.id} className="flex items-center gap-2">
          <Input
            aria-invalid={!!errors.servers?.[index]?.value}
            {...register(`servers.${index}.value` as const)}
          />
          <Button
            type="button"
            size="icon"
            variant="ghost"
            className="shrink-0 text-red-500 hover:text-red-700"
            onClick={() => remove(index)}
            aria-label={`Remove NTP server ${index + 1}`}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ))}

      <div className="flex flex-wrap items-end gap-2">
        <div className="flex flex-col gap-0.5">
          <Input
            placeholder="Add NTP server address"
            aria-invalid={addServerForm.formState.errors.server ? 'true' : undefined}
            aria-describedby={addServerForm.formState.errors.server ? 'ntp-draft-err' : undefined}
            {...addServerForm.register('server')}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                void addServerForm.handleSubmit((d) => {
                  append({ value: d.server });
                  addServerForm.reset({ server: '' });
                })();
              }
            }}
          />
          {addServerForm.formState.errors.server ? (
            <span id="ntp-draft-err" className="text-xs text-red-500" role="alert">
              {addServerForm.formState.errors.server.message}
            </span>
          ) : null}
        </div>
        <Button
          type="button"
          variant="outline"
          onClick={() =>
            addServerForm.handleSubmit((d) => {
              append({ value: d.server });
              addServerForm.reset({ server: '' });
            })()
          }
        >
          Add
        </Button>
      </div>
    </div>
  );
}
