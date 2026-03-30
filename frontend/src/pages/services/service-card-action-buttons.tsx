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

export function ServiceCardActionButtons({
  service,
  isPending,
  onInstall,
  onRemove,
  onStart,
  onStop,
}: ServiceCardActionButtonsProps) {
  return (
    <div className="mt-3 flex flex-wrap gap-2">
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
      {service.id === 'adguardhome' && service.state === 'running' && (
        <Button size="sm" variant="outline" asChild>
          <a
            href={`http://${window.location.hostname}:3000`}
            target="_blank"
            rel="noopener noreferrer"
          >
            <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
            Open Dashboard
          </a>
        </Button>
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
  );
}
