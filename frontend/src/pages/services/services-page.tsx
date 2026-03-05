import { Package } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useServices,
  useInstallService,
  useRemoveService,
  useStartService,
  useStopService,
} from '@/hooks/use-services';
import { ServiceCard } from './service-card';

export function ServicesPage() {
  const { data: services = [], isLoading } = useServices();
  const installMutation = useInstallService();
  const removeMutation = useRemoveService();
  const startMutation = useStartService();
  const stopMutation = useStopService();

  const isPending =
    installMutation.isPending ||
    removeMutation.isPending ||
    startMutation.isPending ||
    stopMutation.isPending;

  return (
    <div className="space-y-6">
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
            <p className="text-sm text-gray-500">No services available</p>
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {services.map((service) => (
                <ServiceCard
                  key={service.id}
                  service={service}
                  onInstall={(id) => installMutation.mutate(id)}
                  onRemove={(id) => removeMutation.mutate(id)}
                  onStart={(id) => startMutation.mutate(id)}
                  onStop={(id) => stopMutation.mutate(id)}
                  isPending={isPending}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
