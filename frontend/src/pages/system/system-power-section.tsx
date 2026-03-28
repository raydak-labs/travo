import { useState } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { useReboot, useShutdown, useFactoryReset } from '@/hooks/use-system';

export function SystemPowerSection() {
  const rebootMutation = useReboot();
  const shutdownMutation = useShutdown();
  const factoryResetMutation = useFactoryReset();
  const [showRebootDialog, setShowRebootDialog] = useState(false);
  const [showShutdownDialog, setShowShutdownDialog] = useState(false);
  const [showFactoryResetDialog, setShowFactoryResetDialog] = useState(false);

  return (
    <>
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-red-500 dark:text-red-400">
          Danger Zone
        </h2>
        <Card className="border-red-200 dark:border-red-900">
          <CardContent className="space-y-4 pt-4">
            <p className="text-xs text-gray-500">
              These actions are irreversible or will cause a service interruption. Proceed with
              caution.
            </p>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="destructive" onClick={() => setShowRebootDialog(true)}>
                Reboot
              </Button>
              <Button size="sm" variant="destructive" onClick={() => setShowShutdownDialog(true)}>
                Shut Down
              </Button>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => setShowFactoryResetDialog(true)}
              >
                Factory Reset
              </Button>
            </div>
            <p className="text-xs text-gray-500">
              Shut Down powers off the device — you will need physical access to turn it back on.
            </p>
          </CardContent>
        </Card>
      </div>

      <ConfirmDialog
        open={showRebootDialog}
        onOpenChange={setShowRebootDialog}
        title="Reboot System"
        description="The router will reboot and be temporarily unreachable."
        warningText="You will lose your connection for 30–60 seconds while the device restarts."
        confirmLabel="Reboot Now"
        isPending={rebootMutation.isPending}
        onConfirm={() => {
          rebootMutation.mutate();
          setShowRebootDialog(false);
        }}
      />

      <ConfirmDialog
        open={showShutdownDialog}
        onOpenChange={setShowShutdownDialog}
        title="Shut Down"
        description="The router will power off. You will need physical access to turn it back on."
        warningText="This will make the router inaccessible until manually powered on."
        confirmLabel="Shut Down"
        isPending={shutdownMutation.isPending}
        onConfirm={() => {
          shutdownMutation.mutate();
          setShowShutdownDialog(false);
        }}
      />

      <ConfirmDialog
        open={showFactoryResetDialog}
        onOpenChange={setShowFactoryResetDialog}
        title="Factory Reset"
        description="This will erase all configuration and restore factory defaults. The device will reboot. You will need to reconnect to the default WiFi network."
        warningText="This action cannot be undone."
        confirmLabel="I understand, Factory Reset"
        isPending={factoryResetMutation.isPending}
        onConfirm={() => {
          factoryResetMutation.mutate();
          setShowFactoryResetDialog(false);
        }}
      />
    </>
  );
}
