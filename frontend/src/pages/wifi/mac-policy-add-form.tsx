import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { macPolicyAddFormSchema, type MacPolicyAddFormValues } from '@/lib/schemas/wifi-forms';

type MACPolicyAddFormProps = {
  onValidSubmit: (data: MacPolicyAddFormValues, onSuccess: () => void) => void;
  isPending: boolean;
};

export function MACPolicyAddForm({ onValidSubmit, isPending }: MACPolicyAddFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<MacPolicyAddFormValues>({
    resolver: zodResolver(macPolicyAddFormSchema),
    defaultValues: { ssid: '', mac: '' },
    mode: 'onChange',
  });

  return (
    <form
      onSubmit={handleSubmit((data) => onValidSubmit(data, () => reset({ ssid: '', mac: '' })))}
      className="space-y-2 rounded-md border p-3"
      noValidate
    >
      <p className="text-xs font-medium text-gray-600 dark:text-gray-400">Add policy</p>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
        <div className="flex flex-1 flex-col gap-0.5">
          <Input
            placeholder="SSID"
            aria-invalid={errors.ssid ? 'true' : undefined}
            aria-describedby={errors.ssid ? 'mac-policy-ssid-err' : undefined}
            className="flex-1"
            {...register('ssid')}
          />
          {errors.ssid ? (
            <span id="mac-policy-ssid-err" className="text-xs text-red-500" role="alert">
              {errors.ssid.message}
            </span>
          ) : null}
        </div>
        <div className="flex flex-1 flex-col gap-0.5">
          <Input
            placeholder="MAC (aa:bb:cc:dd:ee:ff)"
            className="font-mono"
            aria-invalid={errors.mac ? 'true' : undefined}
            aria-describedby={errors.mac ? 'mac-policy-mac-err' : undefined}
            {...register('mac')}
          />
          {errors.mac ? (
            <span id="mac-policy-mac-err" className="text-xs text-red-500" role="alert">
              {errors.mac.message}
            </span>
          ) : null}
        </div>
        <Button
          type="submit"
          size="sm"
          disabled={isPending}
          className="gap-1.5 shrink-0 sm:self-start"
        >
          <Plus className="h-3.5 w-3.5" />
          Add
        </Button>
      </div>
    </form>
  );
}
