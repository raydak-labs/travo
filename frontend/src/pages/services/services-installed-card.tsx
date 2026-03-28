import { Package } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import type { ServiceInfo } from '@shared/index';
import { ServiceCard } from '@/pages/services/service-card';

type ServicesInstalledCardProps = {
  services: ServiceInfo[];
  isLoading: boolean;
  onInstall: (id: string) => void;
  onRemove: (id: string) => void;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
  onAutoStartChange: (id: string, enabled: boolean) => void;
  isPending: boolean;
  isAutoStartPending: boolean;
  streamActionActive: boolean;
};

export function ServicesInstalledCard({
  services,
  isLoading,
  onInstall,
  onRemove,
  onStart,
  onStop,
  onAutoStartChange,
  isPending,
  isAutoStartPending,
  streamActionActive,
}: ServicesInstalledCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Installed Services</CardTitle>
        <Package className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="grid gap-4 md:grid-cols-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-32 w-full" />
            ))}
          </div>
        ) : services.length === 0 ? (
          <EmptyState message="No services available" />
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {services.map((service) => (
              <ServiceCard
                key={service.id}
                service={service}
                onInstall={onInstall}
                onRemove={onRemove}
                onStart={onStart}
                onStop={onStop}
                onAutoStartChange={onAutoStartChange}
                isPending={isPending || streamActionActive}
                isAutoStartPending={isAutoStartPending}
              />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
