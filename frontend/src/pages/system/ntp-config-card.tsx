import { useEffect, useMemo, useState } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useNTPConfig, useSetNTPConfig, useSyncNTP } from '@/hooks/use-system';
import {
  ntpConfigFormSchema,
  ntpServerDraftSchema,
  type NtpConfigFormValues,
  type NtpServerDraftFormValues,
} from '@/lib/schemas/system-forms';
import { NtpConfigSummaryView } from './ntp-config-summary-view';
import { NtpConfigEditForm } from './ntp-config-edit-form';

export function NTPConfigCard() {
  const { data: ntpConfig, isLoading: ntpLoading } = useNTPConfig();
  const setNTPMutation = useSetNTPConfig();
  const syncNTPMutation = useSyncNTP();

  const [isEditing, setIsEditing] = useState(false);

  const addServerForm = useForm<NtpServerDraftFormValues>({
    resolver: zodResolver(ntpServerDraftSchema),
    defaultValues: { server: '' },
    mode: 'onChange',
  });

  const {
    register,
    control,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<NtpConfigFormValues>({
    resolver: zodResolver(ntpConfigFormSchema),
    defaultValues: { enabled: true, servers: [] },
  });

  const { fields, append, remove } = useFieldArray({
    control,
    name: 'servers',
  });

  const ntpEnabled = watch('enabled');
  const serverFields = watch('servers');

  const serversSummary = useMemo(() => {
    const trimmed = serverFields.map((s) => s.value.trim()).filter(Boolean);
    return trimmed.length > 0 ? trimmed.join(', ') : '—';
  }, [serverFields]);

  useEffect(() => {
    if (ntpConfig && !isEditing) {
      reset({
        enabled: ntpConfig.enabled,
        servers: ntpConfig.servers.map((s) => ({ value: s })),
      });
    }
  }, [ntpConfig, isEditing, reset]);

  const openEdit = () => {
    if (ntpConfig) {
      reset({
        enabled: ntpConfig.enabled,
        servers: ntpConfig.servers.length > 0 ? ntpConfig.servers.map((s) => ({ value: s })) : [],
      });
    }
    setIsEditing(true);
  };

  const handleCancel = () => {
    if (ntpConfig) {
      reset({
        enabled: ntpConfig.enabled,
        servers: ntpConfig.servers.map((s) => ({ value: s })),
      });
    }
    addServerForm.reset({ server: '' });
    setIsEditing(false);
  };

  const onSave = (data: NtpConfigFormValues) => {
    const servers = data.servers.map((s) => s.value.trim()).filter(Boolean);
    setNTPMutation.mutate(
      { enabled: data.enabled, servers },
      {
        onSuccess: () => {
          setIsEditing(false);
          addServerForm.reset({ server: '' });
        },
      },
    );
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">NTP Configuration</CardTitle>
        <Clock className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {ntpLoading ? (
          <Skeleton className="h-4 w-1/2" />
        ) : (
          <div className="space-y-4">
            {!isEditing ? (
              <NtpConfigSummaryView
                ntpEnabled={ntpEnabled}
                serversSummary={serversSummary}
                onSync={() => syncNTPMutation.mutate()}
                onEdit={openEdit}
                syncPending={syncNTPMutation.isPending}
                editDisabled={setNTPMutation.isPending}
              />
            ) : (
              <NtpConfigEditForm
                register={register}
                handleSubmit={handleSubmit}
                errors={errors}
                fields={fields}
                append={append}
                remove={remove}
                addServerForm={addServerForm}
                onSave={onSave}
                onCancel={handleCancel}
                onSync={() => syncNTPMutation.mutate()}
                savePending={setNTPMutation.isPending}
                syncPending={syncNTPMutation.isPending}
              />
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
