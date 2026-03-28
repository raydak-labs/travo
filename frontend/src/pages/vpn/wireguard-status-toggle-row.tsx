import { Loader2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import type { VpnStatus } from '@shared/index';

export type WireguardStatusToggleRowProps = {
  wgStatus: VpnStatus | undefined;
  isToggling: boolean;
  desiredEnabled: boolean | undefined;
  toggleMutationPending: boolean;
  onToggleWireguard: () => void;
};

export function WireguardStatusToggleRow({
  wgStatus,
  isToggling,
  desiredEnabled,
  toggleMutationPending,
  onToggleWireguard,
}: WireguardStatusToggleRowProps) {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2">
        <span className="text-sm text-gray-700 dark:text-gray-300">Status</span>
        <Badge variant={wgStatus?.connected ? 'success' : 'outline'}>
          {isToggling
            ? desiredEnabled
              ? 'Enabling…'
              : 'Disabling…'
            : wgStatus?.connected
              ? 'Connected'
              : 'Disconnected'}
        </Badge>
        {isToggling && (
          <span className="inline-flex items-center gap-1 text-xs text-gray-500">
            <Loader2 className="h-3 w-3 animate-spin" />
            Applying changes…
          </span>
        )}
      </div>
      <Switch
        id="wireguard-toggle"
        label="Enable"
        checked={wgStatus?.enabled ?? false}
        onChange={onToggleWireguard}
        disabled={toggleMutationPending}
        aria-busy={isToggling ? 'true' : undefined}
      />
    </div>
  );
}
