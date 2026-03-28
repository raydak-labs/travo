import type {
  UseFormClearErrors,
  UseFormGetValues,
  UseFormSetError,
  UseFormSetValue,
} from 'react-hook-form';
import type { WireguardProfileImportFormValues } from '@/lib/schemas/vpn-forms';

export type WireguardImportFormActions = {
  clearErrors: UseFormClearErrors<WireguardProfileImportFormValues>;
  setValue: UseFormSetValue<WireguardProfileImportFormValues>;
  getValues: UseFormGetValues<WireguardProfileImportFormValues>;
  setError: UseFormSetError<WireguardProfileImportFormValues>;
};

/** Read uploaded .conf into the import form; derive profile name from filename when empty. */
export async function applyWireguardImportFile(
  file: File | null,
  form: WireguardImportFormActions,
): Promise<void> {
  form.clearErrors('config');
  if (!file) return;
  try {
    const text = await file.text();
    form.setValue('config', text, { shouldValidate: true });
    if (!form.getValues('name').trim()) {
      const base = file.name.replace(/\.conf$/i, '');
      form.setValue('name', base || file.name, { shouldValidate: true });
    }
  } catch (e: unknown) {
    form.setError('config', {
      message: e instanceof Error ? e.message : 'Failed to read file',
    });
  }
}
