import type { ServiceInfo, ServiceState } from '@shared/index';
import { Shield, ShieldCheck, ShieldBan, Globe } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';

const serviceIcons: Record<string, typeof Shield> = {
  wireguard: Shield,
  tailscale: ShieldCheck,
  adguardhome: ShieldBan,
  openvpn: Globe,
};

const stateBadgeVariant: Record<ServiceState, 'success' | 'warning' | 'outline' | 'destructive'> = {
  running: 'success',
  installed: 'warning',
  stopped: 'warning',
  not_installed: 'outline',
  error: 'destructive',
};

const stateLabels: Record<ServiceState, string> = {
  running: 'Running',
  installed: 'Installed',
  stopped: 'Stopped',
  not_installed: 'Not Installed',
  error: 'Error',
};

interface ServiceCardProps {
  service: ServiceInfo;
  onInstall: (id: string) => void;
  onRemove: (id: string) => void;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
  isPending: boolean;
}

export function ServiceCard({
  service,
  onInstall,
  onRemove,
  onStart,
  onStop,
  isPending,
}: ServiceCardProps) {
  const Icon = serviceIcons[service.id] ?? Globe;

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start gap-3">
          <div className="rounded-md bg-blue-50 p-2 dark:bg-blue-950">
            <Icon className="h-5 w-5 text-blue-600 dark:text-blue-400" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <h3 className="font-medium text-gray-900 dark:text-white">{service.name}</h3>
              <Badge variant={stateBadgeVariant[service.state]}>{stateLabels[service.state]}</Badge>
            </div>
            <p className="mt-1 text-sm text-gray-500">{service.description}</p>
            {service.version && <p className="mt-0.5 text-xs text-gray-400">v{service.version}</p>}

            {/* Auto-start toggle (only when installed) */}
            {service.state !== 'not_installed' && (
              <div className="mt-2">
                <Switch
                  id={`autostart-${service.id}`}
                  label="Auto-start"
                  checked={service.auto_start}
                  readOnly
                />
              </div>
            )}

            {/* Action buttons */}
            <div className="mt-3 flex flex-wrap gap-2">
              {service.state === 'not_installed' && (
                <Button size="sm" disabled={isPending} onClick={() => onInstall(service.id)}>
                  {isPending ? 'Installing...' : 'Install'}
                </Button>
              )}
              {(service.state === 'installed' || service.state === 'stopped') && (
                <>
                  <Button size="sm" disabled={isPending} onClick={() => onStart(service.id)}>
                    {isPending ? 'Starting...' : 'Start'}
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    disabled={isPending}
                    onClick={() => onRemove(service.id)}
                  >
                    Remove
                  </Button>
                </>
              )}
              {service.state === 'running' && (
                <>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={isPending}
                    onClick={() => onStop(service.id)}
                  >
                    {isPending ? 'Stopping...' : 'Stop'}
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    disabled={isPending}
                    onClick={() => onRemove(service.id)}
                  >
                    Remove
                  </Button>
                </>
              )}
              {service.state === 'error' && (
                <>
                  <Button size="sm" disabled={isPending} onClick={() => onStart(service.id)}>
                    Restart
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    disabled={isPending}
                    onClick={() => onRemove(service.id)}
                  >
                    Remove
                  </Button>
                </>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
