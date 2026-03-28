import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useWifiScan, useWifiConnect } from '@/hooks/use-wifi';
import { SetupWifiNetworkList } from '@/pages/setup/setup-wifi-network-list';
import { setupWifiFormSchema, type SetupWifiFormValues } from '@/pages/setup/setup-schema';
import { WifiStepIntro } from '@/pages/setup/wifi-step-intro';
import { WifiStepPasswordField } from '@/pages/setup/wifi-step-password-field';

export function WifiStep({ onNext, onBack }: { onNext: () => void; onBack: () => void }) {
  const { data: networks, isLoading: scanning, refetch: rescan } = useWifiScan();
  const connectMutation = useWifiConnect();
  const [showPassword, setShowPassword] = useState(false);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = useForm<SetupWifiFormValues>({
    resolver: zodResolver(setupWifiFormSchema),
    defaultValues: {
      selectedSsid: '',
      encryption: 'none',
      wifiPassword: '',
    },
    mode: 'onSubmit',
  });

  const selectedSsid = watch('selectedSsid');
  const selectedNetwork = networks?.find((n) => n.ssid === selectedSsid);

  const onConnect = (data: SetupWifiFormValues) => {
    connectMutation.mutate(
      {
        ssid: data.selectedSsid.trim(),
        password: data.wifiPassword,
        encryption: selectedNetwork?.encryption ?? data.encryption,
      },
      { onSuccess: () => onNext() },
    );
  };

  return (
    <div className="space-y-6">
      <WifiStepIntro />

      <form onSubmit={handleSubmit(onConnect)} className="space-y-6" noValidate>
        <SetupWifiNetworkList
          networks={networks}
          scanning={scanning}
          selectedSsid={selectedSsid}
          onRescan={() => rescan()}
          onPickNetwork={(network) => {
            setValue('selectedSsid', network.ssid, { shouldValidate: true });
            setValue('encryption', network.encryption ?? 'none');
            setValue('wifiPassword', '');
          }}
        />

        {errors.selectedSsid ? (
          <p className="text-sm text-red-500" role="alert">
            {errors.selectedSsid.message}
          </p>
        ) : null}

        {selectedSsid && selectedNetwork?.encryption !== 'none' && (
          <WifiStepPasswordField
            selectedSsid={selectedSsid}
            showPassword={showPassword}
            onTogglePassword={() => setShowPassword((v) => !v)}
            register={register}
            errors={errors}
          />
        )}

        <div className="flex gap-3">
          <Button type="button" variant="outline" onClick={onBack} className="flex-1">
            Back
          </Button>
          <Button type="submit" disabled={connectMutation.isPending} className="flex-1">
            {connectMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Connect
          </Button>
        </div>
      </form>
      <button
        type="button"
        onClick={onNext}
        className="block w-full text-center text-sm text-gray-400 transition-colors hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:hover:text-gray-300"
      >
        Skip for now
      </button>
    </div>
  );
}
