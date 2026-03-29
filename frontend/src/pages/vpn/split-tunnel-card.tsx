import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { GitFork } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useSplitTunnel, useSetSplitTunnel } from '@/hooks/use-vpn';
import { splitTunnelFormSchema, type SplitTunnelFormValues } from '@/lib/schemas/vpn-forms';

export function SplitTunnelCard() {
  const { data: splitTunnel, isLoading } = useSplitTunnel();
  const setSplitTunnel = useSetSplitTunnel();

  const { register, handleSubmit, reset, watch } = useForm<SplitTunnelFormValues>({
    resolver: zodResolver(splitTunnelFormSchema),
    defaultValues: { mode: 'all', routes_text: '' },
    mode: 'onChange',
  });

  const mode = watch('mode');

  useEffect(() => {
    if (splitTunnel) {
      reset({
        mode: splitTunnel.mode,
        routes_text: (splitTunnel.routes ?? []).join(', '),
      });
    }
  }, [splitTunnel, reset]);

  const onSave = (data: SplitTunnelFormValues) => {
    const routes =
      data.mode === 'custom'
        ? data.routes_text
            .split(',')
            .map((r) => r.trim())
            .filter(Boolean)
        : [];
    setSplitTunnel.mutate({ mode: data.mode, routes });
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Split Tunneling</CardTitle>
          <GitFork className="h-4 w-4 text-gray-400" />
        </CardHeader>
        <CardContent>
          <div className="h-16 animate-pulse rounded bg-gray-100 dark:bg-gray-800" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Split Tunneling</CardTitle>
        <GitFork className="h-4 w-4 text-gray-400" />
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onSave)} className="space-y-4" noValidate>
          <p className="text-xs text-gray-500">
            Control which traffic is routed through the WireGuard VPN.
          </p>

          <div className="space-y-2">
            <label className="flex cursor-pointer items-center gap-2">
              <input
                type="radio"
                value="all"
                className="h-4 w-4 accent-blue-600"
                {...register('mode')}
              />
              <span className="text-sm">All traffic through VPN</span>
            </label>
            <label className="flex cursor-pointer items-center gap-2">
              <input
                type="radio"
                value="custom"
                className="h-4 w-4 accent-blue-600"
                {...register('mode')}
              />
              <span className="text-sm">Custom routes only</span>
            </label>
          </div>

          {mode === 'custom' && (
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-gray-600 dark:text-gray-400">
                CIDR ranges (comma-separated)
              </label>
              <textarea
                placeholder="e.g. 10.0.0.0/8, 192.168.1.0/24"
                rows={3}
                className="w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                {...register('routes_text')}
              />
              <p className="text-xs text-gray-400">
                Only traffic matching these CIDRs will be routed through the VPN.
              </p>
            </div>
          )}

          <Button type="submit" size="sm" disabled={setSplitTunnel.isPending}>
            {setSplitTunnel.isPending ? 'Saving…' : 'Save'}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
