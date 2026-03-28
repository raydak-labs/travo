import type {
  FieldErrors,
  UseFormRegister,
  UseFormHandleSubmit,
  UseFieldArrayReturn,
  UseFormReturn,
} from 'react-hook-form';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import type { NtpConfigFormValues, NtpServerDraftFormValues } from '@/lib/schemas/system-forms';
import { NtpConfigServerFields } from '@/pages/system/ntp-config-server-fields';

type NtpConfigEditFormProps = {
  register: UseFormRegister<NtpConfigFormValues>;
  handleSubmit: UseFormHandleSubmit<NtpConfigFormValues>;
  errors: FieldErrors<NtpConfigFormValues>;
  fields: UseFieldArrayReturn<NtpConfigFormValues, 'servers', 'id'>['fields'];
  append: UseFieldArrayReturn<NtpConfigFormValues, 'servers', 'id'>['append'];
  remove: UseFieldArrayReturn<NtpConfigFormValues, 'servers', 'id'>['remove'];
  addServerForm: UseFormReturn<NtpServerDraftFormValues>;
  onSave: (data: NtpConfigFormValues) => void;
  onCancel: () => void;
  onSync: () => void;
  savePending: boolean;
  syncPending: boolean;
};

export function NtpConfigEditForm({
  register,
  handleSubmit,
  errors,
  fields,
  append,
  remove,
  addServerForm,
  onSave,
  onCancel,
  onSync,
  savePending,
  syncPending,
}: NtpConfigEditFormProps) {
  return (
    <form onSubmit={handleSubmit(onSave)} className="space-y-4" noValidate>
      <div className="flex items-center justify-between">
        <label htmlFor="ntp-enabled" className="text-sm text-gray-700 dark:text-gray-300">
          Enable NTP time synchronization
        </label>
        <Switch id="ntp-enabled" label="Enable NTP" {...register('enabled')} />
      </div>

      <NtpConfigServerFields
        register={register}
        errors={errors}
        fields={fields}
        append={append}
        remove={remove}
        addServerForm={addServerForm}
      />

      {errors.servers?.message ? (
        <p className="text-sm text-red-500" role="alert">
          {errors.servers.message}
        </p>
      ) : null}

      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={savePending} size="sm">
          {savePending ? 'Saving…' : 'Save NTP Settings'}
        </Button>

        <Button
          variant="outline"
          size="sm"
          type="button"
          onClick={onSync}
          disabled={syncPending}
          title="Force a one-shot NTP sync with pool.ntp.org"
        >
          {syncPending ? 'Syncing…' : 'Sync Now'}
        </Button>

        <Button variant="outline" size="sm" type="button" onClick={onCancel} disabled={savePending}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
