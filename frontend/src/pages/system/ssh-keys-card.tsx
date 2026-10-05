import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Key } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
import { useSSHKeys, useAddSSHKey, useDeleteSSHKey } from '@/hooks/use-system';
import { sshPublicKeyFormSchema, type SshPublicKeyFormValues } from '@/lib/schemas/system-forms';
import { SSHKeyAddForm } from '@/pages/system/ssh-key-add-form';
import { SSHKeysList } from '@/pages/system/ssh-keys-list';

export function SSHKeysCard() {
  const { data: keys, isLoading, isError, error, refetch } = useSSHKeys();
  const addSSHKey = useAddSSHKey();
  const deleteSSHKey = useDeleteSSHKey();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<SshPublicKeyFormValues>({
    resolver: zodResolver(sshPublicKeyFormSchema),
    defaultValues: { key: '' },
    mode: 'onChange',
  });

  const onSubmit = (data: SshPublicKeyFormValues) => {
    addSSHKey.mutate(
      { key: data.key.trim() },
      {
        onSuccess: () => reset({ key: '' }),
      },
    );
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>SSH Public Keys</CardTitle>
        <Key className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        {/* `keys = []` on failure read as "no keys configured" — an empty
            authoritative-looking list. QueryCard states the failure instead. */}
        <QueryCard
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={() => void refetch()}
          loading={<Skeleton className="h-16 w-full" />}
        >
          <SSHKeysList
            keys={[...(keys ?? [])]}
            deletePending={deleteSSHKey.isPending}
            onDelete={(index) => deleteSSHKey.mutate(index)}
          />
        </QueryCard>

        <SSHKeyAddForm
          register={register}
          errors={errors}
          onSubmit={handleSubmit(onSubmit)}
          addPending={addSSHKey.isPending}
        />
      </CardContent>
    </Card>
  );
}
