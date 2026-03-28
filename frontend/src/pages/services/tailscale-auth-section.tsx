import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ExternalLink } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useTailscaleAuth } from '@/hooks/use-vpn';
import { tailscaleAuthFormSchema, type TailscaleAuthFormValues } from '@/lib/schemas/vpn-forms';

export function TailscaleAuthSection() {
  const authMutation = useTailscaleAuth();
  const { register, handleSubmit } = useForm<TailscaleAuthFormValues>({
    resolver: zodResolver(tailscaleAuthFormSchema),
    defaultValues: { auth_key: '' },
    mode: 'onChange',
  });

  const onSubmit = (data: TailscaleAuthFormValues) => {
    authMutation.mutate(data.auth_key.trim() || undefined);
  };

  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-600 dark:text-gray-400">
        Not authenticated. Enter a pre-auth key or start interactive login.
      </p>
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-wrap gap-2" noValidate>
        <Input
          placeholder="tskey-auth-... (optional)"
          className="min-w-[12rem] flex-1 font-mono text-sm"
          aria-label="Tailscale pre-auth key"
          {...register('auth_key')}
        />
        <Button type="submit" size="sm" disabled={authMutation.isPending}>
          Authenticate
        </Button>
      </form>
      {authMutation.data?.auth_url && (
        <a
          href={authMutation.data.auth_url}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
        >
          Open auth URL <ExternalLink className="h-3 w-3" />
        </a>
      )}
    </div>
  );
}
