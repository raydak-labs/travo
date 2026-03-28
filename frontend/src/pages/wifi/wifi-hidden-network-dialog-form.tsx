import type { Control, FieldErrors, UseFormHandleSubmit, UseFormRegister } from 'react-hook-form';
import { Button } from '@/components/ui/button';
import type { WifiHiddenNetworkFormValues } from '@/lib/schemas/wifi-forms';
import { WifiHiddenNetworkDialogFields } from '@/pages/wifi/wifi-hidden-network-dialog-fields';

type WifiHiddenNetworkDialogFormProps = {
  register: UseFormRegister<WifiHiddenNetworkFormValues>;
  control: Control<WifiHiddenNetworkFormValues>;
  handleSubmit: UseFormHandleSubmit<WifiHiddenNetworkFormValues>;
  onValidSubmit: (data: WifiHiddenNetworkFormValues) => void;
  errors: FieldErrors<WifiHiddenNetworkFormValues>;
  needsPassword: boolean;
  showPassword: boolean;
  setShowPassword: (next: boolean) => void;
  ssidValue: string;
  connectPending: boolean;
  errorMessage: string | null;
  onCancel: () => void;
};

export function WifiHiddenNetworkDialogForm({
  register,
  control,
  handleSubmit,
  onValidSubmit,
  errors,
  needsPassword,
  showPassword,
  setShowPassword,
  ssidValue,
  connectPending,
  errorMessage,
  onCancel,
}: WifiHiddenNetworkDialogFormProps) {
  return (
    <form onSubmit={handleSubmit(onValidSubmit)} className="space-y-4" noValidate>
      <WifiHiddenNetworkDialogFields
        register={register}
        control={control}
        errors={errors}
        needsPassword={needsPassword}
        showPassword={showPassword}
        setShowPassword={setShowPassword}
        errorMessage={errorMessage}
      />
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onCancel} disabled={connectPending}>
          Cancel
        </Button>
        <Button type="submit" disabled={connectPending || !ssidValue.trim()}>
          {connectPending ? 'Connecting...' : 'Connect'}
        </Button>
      </div>
    </form>
  );
}
