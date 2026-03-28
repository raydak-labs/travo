import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';

type FirmwareUpgradeConfirmDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  fileName: string;
  keepSettings: boolean;
  onConfirm: () => void;
  confirmPending: boolean;
};

export function FirmwareUpgradeConfirmDialog({
  open,
  onOpenChange,
  fileName,
  keepSettings,
  onConfirm,
  confirmPending,
}: FirmwareUpgradeConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-red-600 dark:text-red-400">
            <AlertTriangle className="h-5 w-5" />
            Firmware Upgrade
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <p className="text-sm text-gray-700 dark:text-gray-300">
            You are about to flash <strong>{fileName}</strong> onto the device.
            {keepSettings
              ? ' Current settings will be preserved.'
              : ' All settings will be erased.'}
          </p>
          <div className="rounded-md bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950 dark:text-red-200">
            <strong>Do not power off the device during the upgrade.</strong> The device will reboot
            automatically.
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={confirmPending}>
            {confirmPending ? 'Flashing…' : 'Flash Firmware'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
