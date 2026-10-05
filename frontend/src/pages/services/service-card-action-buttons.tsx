import { ExternalLink } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import type { ServiceInfo } from '@shared/index';

type ServiceCardActionButtonsProps = {
  service: ServiceInfo;
  isPending: boolean;
  onInstall: (id: string) => void;
  onRemove: (id: string) => void;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
};

/**
 * Removal is unrecoverable from the UI, and it sits on the same 32px row as
 * Start/Stop — on a 360px phone it wraps directly underneath them and becomes a
 * one-tap mis-fire. It gets its own full-width row on narrow layouts, always
 * last, and names the package so the accessible name is not just "Remove".
 */
function ServiceRemoveButton({
  service,
  isPending,
  onRemove,
}: {
  service: ServiceInfo;
  isPending: boolean;
  onRemove: (id: string) => void;
}) {
  return (
    <div className="w-full basis-full border-t border-gray-200 pt-2 dark:border-white/10">
      <Button
        size="sm"
        variant="destructive"
        disabled={isPending}
        aria-label={`Remove ${service.name}`}
        onClick={() => onRemove(service.id)}
      >
        Remove
      </Button>
    </div>
  );
}

export function ServiceCardActionButtons({
  service,
  isPending,
  onInstall,
  onRemove,
  onStart,
  onStop,
}: ServiceCardActionButtonsProps) {
  const remove = (
    <ServiceRemoveButton key="remove" service={service} isPending={isPending} onRemove={onRemove} />
  );

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      {service.id === 'tailscale' && service.state !== 'not_installed' && (
        <Button size="sm" variant="outline" asChild>
          <Link to="/services/tailscale">Manage</Link>
        </Button>
      )}
      {service.id === 'sqm' && service.state !== 'not_installed' && (
        <Button size="sm" variant="outline" asChild>
          <Link to="/services/sqm">Configure</Link>
        </Button>
      )}
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
          {remove}
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
          {service.id === 'adguardhome' && (
            <Button size="sm" variant="outline" asChild>
              <a
                href={`http://${window.location.hostname}:3000`}
                target="_blank"
                rel="noopener noreferrer"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Open Dashboard
              </a>
            </Button>
          )}
          {remove}
        </>
      )}
      {service.state === 'error' && (
        <>
          <Button size="sm" disabled={isPending} onClick={() => onStart(service.id)}>
            Restart
          </Button>
          {remove}
        </>
      )}
    </div>
  );
}
