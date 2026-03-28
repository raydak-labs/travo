import type { UseFormReturn } from 'react-hook-form';
import { Plus, Upload, FileUp } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';
import type { WireguardProfileImportFormValues } from '@/lib/schemas/vpn-forms';

type WireguardImportProfileFormProps = {
  form: UseFormReturn<WireguardProfileImportFormValues>;
  onSubmit: (data: WireguardProfileImportFormValues) => void;
  onFileSelected: (file: File | null) => void;
  isSaving: boolean;
};

export function WireguardImportProfileForm({
  form,
  onSubmit,
  onFileSelected,
  isSaving,
}: WireguardImportProfileFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors: importErrors },
  } = form;

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-2" noValidate>
      <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300">
        <div className="flex items-center gap-1">
          <Upload className="h-4 w-4" />
          Import Profile
        </div>
      </h4>
      <Input
        id="wg-profile-name"
        label="Profile name"
        placeholder="Profile name (e.g. Home VPN, Travel, Work)"
        aria-invalid={importErrors.name ? 'true' : undefined}
        aria-describedby={importErrors.name ? 'wg-name-err' : undefined}
        className="mb-0"
        {...register('name')}
      />
      {importErrors.name ? (
        <p id="wg-name-err" className="text-xs text-red-500" role="alert">
          {importErrors.name.message}
        </p>
      ) : null}

      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          className="gap-1.5"
          onClick={() => document.getElementById('wg-file')?.click()}
          type="button"
        >
          <FileUp className="h-4 w-4" />
          Upload .conf
        </Button>
        <input
          id="wg-file"
          type="file"
          accept=".conf,text/plain"
          className="hidden"
          onChange={(e) => void onFileSelected(e.target.files?.[0] ?? null)}
        />
        <span className="text-xs text-gray-500">or paste below</span>
      </div>
      <textarea
        className={cn(
          'w-full rounded-md border bg-white p-2 text-sm font-mono dark:bg-gray-900 dark:text-white',
          importErrors.config
            ? 'border-red-500 dark:border-red-500'
            : 'border-gray-300 dark:border-gray-700',
        )}
        rows={4}
        placeholder="Paste WireGuard config here..."
        aria-invalid={importErrors.config ? 'true' : undefined}
        aria-describedby={importErrors.config ? 'wg-config-err' : undefined}
        {...register('config')}
      />
      {importErrors.config ? (
        <p id="wg-config-err" className="text-sm text-red-600 dark:text-red-400" role="alert">
          {importErrors.config.message}
        </p>
      ) : null}
      <Button type="submit" size="sm" disabled={isSaving}>
        <Plus className="mr-1 h-4 w-4" />
        {isSaving ? 'Saving...' : 'Save Profile'}
      </Button>
    </form>
  );
}
