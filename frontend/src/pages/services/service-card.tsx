import type { ServiceInfo } from '@shared/index';
import { Globe } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { QueryCard } from '@/components/ui/query-card';
import { StatusPill } from '@/components/ui/status-pill';
import { Switch } from '@/components/ui/switch';
import { useAdGuardDNS, useSetAdGuardDNS } from '@/hooks/use-services';
import { serviceCardIcons, serviceStateTone, serviceStateLabels } from './service-card.constants';
import { ServiceCardActionButtons } from './service-card-action-buttons';

interface ServiceCardProps {
  service: ServiceInfo;
  onInstall: (id: string) => void;
  onRemove: (id: string) => void;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
  onAutoStartChange: (id: string, enabled: boolean) => void;
  isPending: boolean;
  isAutoStartPending: boolean;
}

export function ServiceCard({
  service,
  onInstall,
  onRemove,
  onStart,
  onStop,
  onAutoStartChange,
  isPending,
  isAutoStartPending,
}: ServiceCardProps) {
  const Icon = serviceCardIcons[service.id] ?? Globe;
  const isAdGuardRunning = service.id === 'adguardhome' && service.state === 'running';
  const dnsQuery = useAdGuardDNS();
  const dnsStatus = dnsQuery.data;
  const setDNS = useSetAdGuardDNS();

  return (
    <Card className="shadow-none hover:shadow-none">
      <CardHeader className="flex flex-row items-start gap-3 space-y-0 pb-2">
        <div className="rounded-md bg-[var(--status-info-surface)] p-2">
          <Icon className="h-5 w-5 text-[var(--status-info-text)]" />
        </div>
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex items-center gap-2">
            <CardTitle>{service.name}</CardTitle>
            <StatusPill tone={serviceStateTone[service.state]} withDot>
              {serviceStateLabels[service.state]}
            </StatusPill>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400">{service.description}</p>
          {service.version && (
            <p className="text-xs text-gray-400 dark:text-gray-500">v{service.version}</p>
          )}
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {service.state !== 'not_installed' && (
          <div className="mt-2">
            <Switch
              id={`autostart-${service.id}`}
              label="Auto-start"
              checked={service.auto_start}
              disabled={isAutoStartPending}
              onChange={() => onAutoStartChange(service.id, !service.auto_start)}
            />
          </div>
        )}

        {isAdGuardRunning && (
          <div className="mt-2">
            {/* Without an error state a failed DNS read dropped the toggle,
                reading as "AdGuard does not handle LAN DNS". */}
            <QueryCard
              isLoading={dnsQuery.isLoading}
              isError={dnsQuery.isError}
              error={dnsQuery.error}
              onRetry={() => void dnsQuery.refetch()}
            >
              {dnsStatus ? (
                <>
                  <Switch
                    id="adguard-dns-toggle"
                    label="DNS Filtering Active"
                    checked={dnsStatus.enabled}
                    disabled={setDNS.isPending}
                    onChange={() => setDNS.mutate(!dnsStatus.enabled)}
                  />
                  <p className="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                    {dnsStatus.enabled
                      ? `Forwarding LAN DNS to AdGuard (port ${dnsStatus.dns_port})`
                      : 'AdGuard is not handling LAN DNS queries'}
                  </p>
                </>
              ) : null}
            </QueryCard>
          </div>
        )}

        <ServiceCardActionButtons
          service={service}
          isPending={isPending}
          onInstall={onInstall}
          onRemove={onRemove}
          onStart={onStart}
          onStop={onStop}
        />
      </CardContent>
    </Card>
  );
}
