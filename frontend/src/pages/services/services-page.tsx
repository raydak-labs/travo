import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Package } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import {
  useServices,
  useInstallService,
  useRemoveService,
  useStartService,
  useStopService,
  useSetAutoStart,
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
  const setAutoStartMutation = useSetAutoStart();
  const queryClient = useQueryClient();

  const [streamAction, setStreamAction] = useState<StreamAction | null>(null);
  const [showWireguardWizard, setShowWireguardWizard] = useState(false);
  const navigate = useNavigate();

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
    const justInstalledWireguard =
      streamAction?.serviceId === 'wireguard' && streamAction?.action === 'install';
    setStreamAction(null);
    void queryClient.invalidateQueries({ queryKey: ['services'] });
    if (justInstalledWireguard) {
      setShowWireguardWizard(true);
    }
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
            <EmptyState message="No services available" />
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
                  onAutoStartChange={(id, enabled) => setAutoStartMutation.mutate({ id, enabled })}
                  isPending={isPending || streamAction !== null}
                  isAutoStartPending={setAutoStartMutation.isPending}
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

      {/* WireGuard post-install wizard */}
      <Dialog open={showWireguardWizard} onOpenChange={setShowWireguardWizard}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>WireGuard Installed!</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-gray-700 dark:text-gray-300">
            WireGuard has been installed. Would you like to set up a VPN configuration now?
          </p>
          <DialogFooter className="flex-col gap-2 sm:flex-row">
            <Button
              variant="outline"
              onClick={() => setShowWireguardWizard(false)}
            >
              Later
            </Button>
            <Button
              onClick={() => {
                setShowWireguardWizard(false);
                void navigate({ to: '/vpn' });
              }}
            >
              Import .conf File
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
