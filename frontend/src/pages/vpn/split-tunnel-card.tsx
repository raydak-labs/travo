import { useState, useEffect } from 'react';
import { GitFork } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useSplitTunnel, useSetSplitTunnel } from '@/hooks/use-vpn';

export function SplitTunnelCard() {
  const { data: splitTunnel, isLoading } = useSplitTunnel();
  const setSplitTunnel = useSetSplitTunnel();

  const [mode, setMode] = useState<'all' | 'custom'>('all');
  const [routesText, setRoutesText] = useState('');

  useEffect(() => {
    if (splitTunnel) {
      setMode(splitTunnel.mode);
      setRoutesText((splitTunnel.routes ?? []).join(', '));
    }
  }, [splitTunnel]);

  const handleSave = () => {
    const routes =
      mode === 'custom'
        ? routesText
            .split(',')
            .map((r) => r.trim())
            .filter(Boolean)
        : [];
    setSplitTunnel.mutate({ mode, routes });
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
      <CardContent className="space-y-4">
        <p className="text-xs text-gray-500">
          Control which traffic is routed through the WireGuard VPN.
        </p>

        <div className="space-y-2">
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="radio"
              name="split-tunnel-mode"
              value="all"
              checked={mode === 'all'}
              onChange={() => setMode('all')}
              className="h-4 w-4 accent-blue-600"
            />
            <span className="text-sm">All traffic through VPN</span>
          </label>
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="radio"
              name="split-tunnel-mode"
              value="custom"
              checked={mode === 'custom'}
              onChange={() => setMode('custom')}
              className="h-4 w-4 accent-blue-600"
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
              value={routesText}
              onChange={(e) => setRoutesText(e.target.value)}
              placeholder="e.g. 10.0.0.0/8, 192.168.1.0/24"
              rows={3}
              className="w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
            <p className="text-xs text-gray-400">
              Only traffic matching these CIDRs will be routed through the VPN.
            </p>
          </div>
        )}

        <Button
          size="sm"
          onClick={handleSave}
          disabled={setSplitTunnel.isPending}
        >
          {setSplitTunnel.isPending ? 'Saving…' : 'Save'}
        </Button>
      </CardContent>
    </Card>
  );
}
