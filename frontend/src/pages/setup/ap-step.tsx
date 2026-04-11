import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import type { APConfigUpdate } from '@shared/index';
import { API_ROUTES } from '@shared/index';
import type { WifiMutationResponse } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useAPConfigs } from '@/hooks/use-wifi';
import { apiClient } from '@/lib/api-client';
import { finalizeWifiMutation } from '@/lib/wifi-apply';
import { APStepCredentialsFields } from '@/pages/setup/ap-step-credentials-fields';
import { APStepIntro } from '@/pages/setup/ap-step-intro';
import { setupApFormSchema, type SetupApFormValues } from '@/pages/setup/setup-schema';

export function APStep({ onNext, onBack }: { onNext: () => void; onBack: () => void }) {
  const queryClient = useQueryClient();
  const { data: apConfigs, isLoading } = useAPConfigs();
  const [showPassword, setShowPassword] = useState(false);
  const seeded = useRef(false);

  const firstAP = apConfigs?.[0];

  const saveAllAPs = useMutation({
    mutationFn: async (data: SetupApFormValues) => {
      if (!apConfigs?.length) {
        throw new Error('No access point configuration available');
      }
      for (const ap of apConfigs) {
        const config: APConfigUpdate = {
          ssid: data.ssid,
          key: data.key,
          encryption: ap.encryption,
          enabled: ap.enabled,
        };
        await finalizeWifiMutation(
          apiClient.put<WifiMutationResponse>(`${API_ROUTES.wifi.ap}/${ap.section}`, config),
        );
      }
    },
    onSuccess: () => {
      toast.success('AP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'ap'] });
      onNext();
    },
    onError: (error: Error) => {
      toast.error('Failed to update AP config', { description: error.message });
    },
  });

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<SetupApFormValues>({
    resolver: zodResolver(setupApFormSchema),
    defaultValues: { ssid: '', key: '' },
    mode: 'onChange',
  });

  useEffect(() => {
    if (firstAP && !seeded.current) {
      reset({
        ssid: firstAP.ssid ?? '',
        key: firstAP.key ?? '',
      });
      seeded.current = true;
    }
  }, [firstAP, reset]);

  const onSave = (data: SetupApFormValues) => {
    saveAllAPs.mutate(data);
  };

  return (
    <div className="space-y-6">
      <APStepIntro />

      {isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : (
        <form onSubmit={handleSubmit(onSave)} className="space-y-4" noValidate>
          <APStepCredentialsFields
            register={register}
            errors={errors}
            showPassword={showPassword}
            onTogglePassword={() => setShowPassword((v) => !v)}
          />

          <div className="flex gap-3">
            <Button type="button" variant="outline" onClick={onBack} className="flex-1">
              Back
            </Button>
            <Button
              type="submit"
              disabled={saveAllAPs.isPending || isLoading || !firstAP}
              className="flex-1"
            >
              {saveAllAPs.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Save AP Config
            </Button>
          </div>
        </form>
      )}
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
