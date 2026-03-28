import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Key } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useSSHKeys, useAddSSHKey, useDeleteSSHKey } from '@/hooks/use-system';
import { sshPublicKeyFormSchema, type SshPublicKeyFormValues } from '@/lib/schemas/system-forms';
import { SSHKeyAddForm } from '@/pages/system/ssh-key-add-form';
import { SSHKeysList } from '@/pages/system/ssh-keys-list';

export function SSHKeysCard() {
  const { data: keys = [], isLoading } = useSSHKeys();
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
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Key className="h-5 w-5" />
          SSH Public Keys
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : (
          <SSHKeysList
            keys={[...keys]}
            deletePending={deleteSSHKey.isPending}
            onDelete={(index) => deleteSSHKey.mutate(index)}
          />
        )}

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
