import { useState, useRef } from 'react';
import { Zap, Upload, AlertTriangle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { useFirmwareUpgrade } from '@/hooks/use-system';
import { formatBytes } from '@/lib/utils';

export function FirmwareUpgradeCard() {
  const firmwareUpgrade = useFirmwareUpgrade();
  const firmwareInputRef = useRef<HTMLInputElement>(null);
  const [firmwareFile, setFirmwareFile] = useState<File | null>(null);
  const [keepSettings, setKeepSettings] = useState(true);
  const [showFirmwareDialog, setShowFirmwareDialog] = useState(false);

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Firmware Upgrade</CardTitle>
          <Zap className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <p className="text-xs text-gray-500">
              Upload a sysupgrade firmware image (.bin) to flash the device.
            </p>
            <div>
              <input
                type="file"
                ref={firmwareInputRef}
                accept=".bin"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) setFirmwareFile(file);
                  e.target.value = '';
                }}
              />
              <Button variant="outline" size="sm" onClick={() => firmwareInputRef.current?.click()}>
                <Upload className="mr-2 h-4 w-4" />
                Select Firmware Image
              </Button>
              {firmwareFile && (
                <p className="mt-1 text-xs text-gray-700 dark:text-gray-300">
                  Selected: {firmwareFile.name} ({formatBytes(firmwareFile.size)})
                </p>
              )}
            </div>
            <div className="flex items-center justify-between">
              <label htmlFor="keep-settings" className="text-sm text-gray-700 dark:text-gray-300">
                Keep current settings
              </label>
              <Switch
                id="keep-settings"
                label="Keep settings"
                checked={keepSettings}
                onChange={(e) => setKeepSettings(e.target.checked)}
              />
            </div>
            <Button
              variant="destructive"
              size="sm"
              disabled={!firmwareFile || firmwareUpgrade.isPending}
              onClick={() => setShowFirmwareDialog(true)}
            >
              <Zap className="mr-2 h-4 w-4" />
              {firmwareUpgrade.isPending ? 'Flashing…' : 'Upload & Flash'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Dialog open={showFirmwareDialog} onOpenChange={setShowFirmwareDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-red-600">
              <AlertTriangle className="h-5 w-5" />
              Firmware Upgrade
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <p className="text-sm text-gray-700 dark:text-gray-300">
              You are about to flash <strong>{firmwareFile?.name}</strong> onto the device.
              {keepSettings
                ? ' Current settings will be preserved.'
                : ' All settings will be erased.'}
            </p>
            <div className="rounded-md bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950 dark:text-red-200">
              <strong>Do not power off the device during the upgrade.</strong> The device will
              reboot automatically.
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowFirmwareDialog(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (firmwareFile) {
                  firmwareUpgrade.mutate(
                    { file: firmwareFile, keepSettings },
                    { onSuccess: () => setFirmwareFile(null) },
                  );
                }
                setShowFirmwareDialog(false);
              }}
              disabled={firmwareUpgrade.isPending}
            >
              {firmwareUpgrade.isPending ? 'Flashing…' : 'Flash Firmware'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
