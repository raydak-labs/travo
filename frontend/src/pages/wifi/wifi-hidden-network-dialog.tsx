import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { useWifiConnect } from '@/hooks/use-wifi';
import {
  wifiHiddenNetworkFormSchema,
  type WifiHiddenNetworkFormValues,
} from '@/lib/schemas/wifi-forms';
import { wifiHiddenNetworkDefaultValues } from './wifi-hidden-network-constants';
import { WifiHiddenNetworkDialogForm } from './wifi-hidden-network-dialog-form';

export function WifiHiddenNetworkDialog() {
  const [open, setOpen] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const connectMutation = useWifiConnect();

  const {
    register,
    control,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<WifiHiddenNetworkFormValues>({
    resolver: zodResolver(wifiHiddenNetworkFormSchema),
    defaultValues: wifiHiddenNetworkDefaultValues,
    mode: 'onChange',
  });

  const encryption = watch('encryption');
  const needsPassword = encryption !== 'none';
  const ssidValue = watch('ssid');

  function resetForm() {
    reset(wifiHiddenNetworkDefaultValues);
    setShowPassword(false);
  }

  function handleOpen() {
    resetForm();
    setOpen(true);
  }

  const onValidSubmit = (data: WifiHiddenNetworkFormValues) => {
    connectMutation.mutate(
      {
        ssid: data.ssid.trim(),
        password: data.password,
        encryption: data.encryption,
        hidden: true,
      },
      {
        onSuccess: () => {
          resetForm();
          setOpen(false);
        },
      },
    );
  };

  const errorMessage =
    connectMutation.error && 'message' in connectMutation.error
      ? String((connectMutation.error as { message: string }).message)
      : null;

  return (
    <>
      <Button onClick={handleOpen} size="sm" variant="outline">
        <WifiOff className="mr-1.5 h-3.5 w-3.5" />
        Hidden Network
      </Button>

      <Dialog
        open={open}
        onOpenChange={(v) => {
          setOpen(v);
          if (!v) resetForm();
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Connect to Hidden Network</DialogTitle>
            <DialogDescription>
              Enter the network name and credentials for a hidden WiFi network.
            </DialogDescription>
          </DialogHeader>

          <WifiHiddenNetworkDialogForm
            register={register}
            control={control}
            handleSubmit={handleSubmit}
            onValidSubmit={onValidSubmit}
            errors={errors}
            needsPassword={needsPassword}
            showPassword={showPassword}
            setShowPassword={setShowPassword}
            ssidValue={ssidValue}
            connectPending={connectMutation.isPending}
            errorMessage={errorMessage}
            onCancel={() => setOpen(false)}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}
