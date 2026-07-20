import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Button } from '@/components/ui/button';
import type { SshPublicKeyFormValues } from '@/lib/schemas/system-forms';

type SSHKeyAddFormProps = {
  register: UseFormRegister<SshPublicKeyFormValues>;
  errors: FieldErrors<SshPublicKeyFormValues>;
  onSubmit: () => void;
  addPending: boolean;
};

export function SSHKeyAddForm({ register, errors, onSubmit, addPending }: SSHKeyAddFormProps) {
  return (
    <form onSubmit={onSubmit} className="space-y-2 pt-2" noValidate>
      <p className="text-sm font-medium">Add a new public key</p>
      <textarea
        className="h-24 w-full rounded-md border border-gray-300 bg-white px-3 py-2 font-mono text-xs text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder:text-gray-500"
        placeholder="ssh-ed25519 AAAA… user@host"
        spellCheck={false}
        aria-invalid={!!errors.key}
        aria-describedby={errors.key ? 'ssh-key-error' : undefined}
        {...register('key')}
      />
      {errors.key ? (
        <p id="ssh-key-error" className="text-xs text-red-500" role="alert">
          {errors.key.message}
        </p>
      ) : null}
      <Button type="submit" disabled={addPending} className="w-full sm:w-auto">
        {addPending ? 'Adding…' : 'Add Key'}
      </Button>
    </form>
  );
}
