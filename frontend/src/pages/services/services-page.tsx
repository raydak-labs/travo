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
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

interface StreamAction {
  serviceId: string;
  serviceName: string;
  action: 'install' | 'remove';
}

export function ServicesPage() {
  const servicesQuery = useServices();
  const services = servicesQuery.data ?? [];
  const installMutation = useInstallService();
  const removeMutation = useRemoveService();
  const startMutation = useStartService();
  const stopMutation = useStopService();
  const setAutoStartMutation = useSetAutoStart();
  const queryClient = useQueryClient();

  const [streamAction, setStreamAction] = useState<StreamAction | null>(null);
  const [pendingRemove, setPendingRemove] = useState<StreamAction | null>(null);
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

  // Removal is not reversible from the UI and its log dialog starts the
  // uninstall stream the moment it mounts, so it needs an explicit confirm
  // before anything is sent to the router.
  const handleRemove = (id: string) => {
    const service = services.find((s) => s.id === id);
    setPendingRemove({
      serviceId: id,
      serviceName: service?.name ?? id,
      action: 'remove',
    });
  };

  const confirmRemove = () => {
    if (!pendingRemove) return;
    setStreamAction(pendingRemove);
    setPendingRemove(null);
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
      <ConfirmDialog
        open={pendingRemove !== null}
        onOpenChange={(open) => {
          if (!open) setPendingRemove(null);
        }}
        title={`Remove ${pendingRemove?.serviceName ?? 'package'}?`}
        description={`This uninstalls ${pendingRemove?.serviceName ?? 'the package'} from the router.`}
        warningText="Removing it deletes the package and its configuration from the router, and the installation log dialog starts the uninstall immediately after you confirm. There is no undo in this UI — reinstall and reconfigure it if you change your mind."
        confirmLabel="Remove now"
        onConfirm={confirmRemove}
      />

      <ServicesInstalledCard
        services={services}
        isLoading={servicesQuery.isLoading}
        isError={servicesQuery.isError}
        error={servicesQuery.error}
        onRetry={() => void servicesQuery.refetch()}
        onInstall={handleInstall}
        onRemove={handleRemove}
        onStart={(id) => startMutation.mutate(id)}
        onStop={(id) => stopMutation.mutate(id)}
        onAutoStartChange={(id, enabled) => setAutoStartMutation.mutate({ id, enabled })}
        isPending={isPending}
        isAutoStartPending={setAutoStartMutation.isPending}
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
