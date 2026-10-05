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
import { routeWithSegment } from '@/lib/api-url';
import {
  ApApplyRollbackError,
  describeApApplyRollback,
  rollbackApSections,
  snapshotApSections,
  type ApSectionSnapshot,
} from '@/lib/ap-section-apply';
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
      const snapshots = snapshotApSections(apConfigs);
      const written: ApSectionSnapshot[] = [];
      for (const snapshot of snapshots) {
        const config: APConfigUpdate = {
          ssid: data.ssid,
          key: data.key,
          encryption: snapshot.config.encryption,
          enabled: snapshot.config.enabled,
        };
        try {
          await finalizeWifiMutation(
            apiClient.put<WifiMutationResponse>(
              routeWithSegment(API_ROUTES.wifi.ap, snapshot.section),
              config,
            ),
          );
          written.push(snapshot);
        } catch (error) {
          // The bands already written are committed on the device; undo them so
          // first-run setup cannot leave the router with two different SSIDs.
          const rollback = await rollbackApSections(written, (previous) =>
            finalizeWifiMutation(
              apiClient.put<WifiMutationResponse>(
                routeWithSegment(API_ROUTES.wifi.ap, previous.section),
                previous.config,
              ),
            ),
          );
          throw new ApApplyRollbackError(snapshot, error, rollback);
        }
      }
    },
    onSuccess: () => {
      toast.success('AP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'ap'] });
      onNext();
    },
    onError: (error: Error) => {
      toast.error('Failed to update AP config', {
        description: describeApApplyRollback(error),
      });
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
    mode: 'onTouched',
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
          <fieldset disabled={saveAllAPs.isPending} className="space-y-4 disabled:opacity-60">
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
                {saveAllAPs.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                Save AP Config
              </Button>
            </div>
          </fieldset>
        </form>
      )}
      <button
        type="button"
        onClick={onNext}
        className="block w-full text-center text-sm text-gray-500 transition-colors hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-gray-400 dark:hover:text-gray-300"
      >
        Skip for now
      </button>
    </div>
  );
}
