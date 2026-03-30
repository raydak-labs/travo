import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useServices,
  useInstallService,
  useRemoveService,
  useStartService,
  useStopService,
  useSetAutoStart,
} from '@/hooks/use-services';
import { InstallLogDialog } from '@/pages/services/install-log-dialog';
import { ServicesInstalledCard } from '@/pages/services/services-installed-card';
import { WireguardPostInstallDialog } from '@/pages/services/wireguard-post-install-dialog';
import { SQMSection } from '@/pages/services/sqm-section';

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
      <ServicesInstalledCard
        services={services}
        isLoading={isLoading}
        onInstall={handleInstall}
        onRemove={handleRemove}
        onStart={(id) => startMutation.mutate(id)}
        onStop={(id) => stopMutation.mutate(id)}
        onAutoStartChange={(id, enabled) => setAutoStartMutation.mutate({ id, enabled })}
        isPending={isPending}
        isAutoStartPending={setAutoStartMutation.isPending}
        streamActionActive={streamAction !== null}
      />

      <SQMSection
        sqmService={services.find((s) => s.id === 'sqm')}
        onInstall={handleInstall}
        streamActionActive={streamAction !== null}
      />

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

      <WireguardPostInstallDialog
        open={showWireguardWizard}
        onOpenChange={setShowWireguardWizard}
      />
    </div>
  );
}
