import { useState } from 'react';
import { Package, ExternalLink, FileEdit } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  useServices,
  useInstallService,
  useRemoveService,
  useStartService,
  useStopService,
  useSetAutoStart,
  useAdGuardConfig,
  useSetAdGuardConfig,
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
  const adguardConfigQuery = useAdGuardConfig();
  const setAdGuardConfig = useSetAdGuardConfig();

  const [streamAction, setStreamAction] = useState<StreamAction | null>(null);
  const [configEditorOpen, setConfigEditorOpen] = useState(false);
  const [configContent, setConfigContent] = useState('');

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

  const adguardRunning = services.some((s) => s.id === 'adguardhome' && s.state === 'running');
  const adguardInstalled = services.some(
    (s) => s.id === 'adguardhome' && s.state !== 'not_installed',
  );

  const handleOpenConfigEditor = async () => {
    const result = await adguardConfigQuery.refetch();
    if (result.data) {
      setConfigContent(result.data.content);
      setConfigEditorOpen(true);
    }
  };

  const handleSaveConfig = () => {
    setAdGuardConfig.mutate(configContent, {
      onSuccess: () => setConfigEditorOpen(false),
    });
  };

  return (
    <div className="space-y-6">
      {/* Quick Links */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Quick Links</CardTitle>
          <ExternalLink className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" asChild>
              <a
                href={`http://${window.location.hostname}/cgi-bin/luci`}
                target="_blank"
                rel="noopener noreferrer"
              >
                <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                LuCI Web Interface
              </a>
            </Button>
            {adguardRunning && (
              <Button size="sm" variant="outline" asChild>
                <a
                  href={`http://${window.location.hostname}:3000`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                  AdGuard Dashboard
                </a>
              </Button>
            )}
            {adguardInstalled && (
              <Button
                size="sm"
                variant="outline"
                onClick={handleOpenConfigEditor}
                disabled={adguardConfigQuery.isFetching}
              >
                <FileEdit className="mr-1.5 h-3.5 w-3.5" />
                {adguardConfigQuery.isFetching ? 'Loading…' : 'AdGuard Config'}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

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

      <Dialog open={configEditorOpen} onOpenChange={setConfigEditorOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>AdGuard Home Configuration</DialogTitle>
            <DialogDescription>
              Edit the AdGuardHome.yaml configuration file. The service will be restarted after
              saving.
            </DialogDescription>
          </DialogHeader>
          <textarea
            className="h-96 w-full rounded-md border border-gray-300 bg-white p-3 font-mono text-sm text-gray-900 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            value={configContent}
            onChange={(e) => setConfigContent(e.target.value)}
            spellCheck={false}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfigEditorOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveConfig} disabled={setAdGuardConfig.isPending}>
              {setAdGuardConfig.isPending ? 'Saving…' : 'Save & Restart'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
