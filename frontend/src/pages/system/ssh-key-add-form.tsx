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
        className="h-24 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        placeholder="ssh-ed25519 AAAA… user@host"
        spellCheck={false}
        aria-invalid={!!errors.key}
        aria-describedby={errors.key ? 'ssh-key-error' : undefined}
        {...register('key')}
      />
      {errors.key ? (
        <p id="ssh-key-error" className="text-xs text-destructive" role="alert">
          {errors.key.message}
        </p>
      ) : null}
      <Button type="submit" disabled={addPending} className="w-full sm:w-auto">
        {addPending ? 'Adding…' : 'Add Key'}
      </Button>
    </form>
  );
}
