import { useState } from 'react';
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
import { InstallLogDialog } from './install-log-dialog';
import { useQueryClient } from '@tanstack/react-query';

interface StreamAction {
  serviceId: string;
  serviceName: string;
  action: 'install' | 'remove';
}

export function ServicesPage() {
  const { data: services = [], isLoading } = useServices();
  const installMutation = useInstallService();
  const removeMutation = useRemoveService();
  const startMutation = useStartService();
  const stopMutation = useStopService();
  const queryClient = useQueryClient();

  const [streamAction, setStreamAction] = useState<StreamAction | null>(null);

  const isPending =
    installMutation.isPending ||
    removeMutation.isPending ||
    startMutation.isPending ||
    stopMutation.isPending;

  const handleInstall = (id: string) => {
    const service = services.find((s) => s.id === id);
    setStreamAction({ serviceId: id, serviceName: service?.name ?? id, action: 'install' });
  };

  const handleRemove = (id: string) => {
    const service = services.find((s) => s.id === id);
    setStreamAction({ serviceId: id, serviceName: service?.name ?? id, action: 'remove' });
  };

  const handleStreamComplete = () => {
    setStreamAction(null);
    void queryClient.invalidateQueries({ queryKey: ['services'] });
  };

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
                  onInstall={handleInstall}
                  onRemove={handleRemove}
                  onStart={(id) => startMutation.mutate(id)}
                  onStop={(id) => stopMutation.mutate(id)}
                  isPending={isPending || streamAction !== null}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {streamAction && (
        <InstallLogDialog
          open={true}
          onOpenChange={(open) => !open && handleStreamComplete()}
          serviceId={streamAction.serviceId}
          serviceName={streamAction.serviceName}
          action={streamAction.action}
          onComplete={handleStreamComplete}
        />
      )}
    </div>
  );
}
