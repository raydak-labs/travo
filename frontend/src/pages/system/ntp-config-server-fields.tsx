import type {
  FieldErrors,
  UseFormRegister,
  UseFieldArrayReturn,
  UseFormReturn,
} from 'react-hook-form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
      <label className="text-xs text-gray-500">NTP Servers</label>
      {fields.map((field, index) => (
        <div key={field.id} className="flex items-center gap-2">
          <Input
            className="h-8 text-sm"
            aria-invalid={!!errors.servers?.[index]?.value}
            {...register(`servers.${index}.value` as const)}
          />
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-8 px-2 text-red-500 hover:text-red-700"
            onClick={() => remove(index)}
          >
            Remove
          </Button>
        </div>
      ))}

      <div className="flex flex-wrap items-end gap-2">
        <div className="flex flex-col gap-0.5">
          <Input
            className="h-8 text-sm"
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
          size="sm"
          variant="outline"
          className="h-8"
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
