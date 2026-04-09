import { useState } from 'react';
import { AlertTriangle, Radio } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useConnectionMethod } from '@/hooks/use-network';

interface ConfirmRadioDisableDialogProps {
  open: boolean;
  radioName: string;
  isPending: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export function ConfirmRadioDisableDialog({
  open,
  radioName,
  isPending,
  onOpenChange,
  onConfirm,
}: ConfirmRadioDisableDialogProps) {
  const { data: connectionMethod } = useConnectionMethod();
  const isWifiClient = connectionMethod?.method === 'wifi-client';

  const [confirmText, setConfirmText] = useState('');
  const isConfirmed = confirmText === 'CONFIRM';

  function handleConfirm() {
    onConfirm();
    setConfirmText('');
  }

  function handleClose() {
    onOpenChange(false);
    setConfirmText('');
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-red-600">
            <AlertTriangle className="h-5 w-5" />
            Disable WiFi Radio
          </DialogTitle>
          <DialogDescription>
            You are about to disable the radio{' '}
            <span className="font-medium text-gray-900 dark:text-white">{radioName}</span>.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="rounded-lg border border-red-300 bg-red-50 p-4 dark:border-red-700 dark:bg-red-950">
            <div className="flex items-start gap-3">
              <Radio className="mt-0.5 h-5 w-5 shrink-0 text-red-600 dark:text-red-400" />
              <div className="flex-1">
                <h3 className="text-sm font-semibold text-red-900 dark:text-red-100">
                  All WiFi will stop working
                </h3>
                <p className="mt-1 text-sm text-red-800 dark:text-red-200">
                  Disabling this radio will turn off all WiFi functionality. This includes:
                </p>
                <ul className="mt-2 space-y-1 text-sm text-red-700 dark:text-red-300">
                  <li>• WiFi client connections (uplink)</li>
                  <li>• WiFi access points (for connecting devices)</li>
                  <li>• Guest WiFi and any wireless features</li>
                </ul>
              </div>
            </div>
          </div>

          {isWifiClient && (
            <div className="rounded-lg border-2 border-red-500 bg-red-100 p-4 dark:bg-red-900">
              <div className="flex items-start gap-3">
                <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-red-600 dark:text-red-400" />
                <div>
                  <h3 className="text-sm font-bold text-red-900 dark:text-red-100">
                    You will lose all access to this device
                  </h3>
                  <p className="mt-1 text-sm text-red-800 dark:text-red-200">
                    You are currently connected via WiFi client. Disabling the radio will cut off
                    your connection immediately.
                  </p>
                  <p className="mt-2 text-sm text-red-800 dark:text-red-200">
                    <strong>Recovery options:</strong>
                  </p>
                  <ul className="mt-1 space-y-1 text-sm text-red-700 dark:text-red-300">
                    <li>• Connect via Ethernet cable to LAN port</li>
                    <li>• Reboot and use Emergency AP (if enabled)</li>
                    <li>• Access via serial console (advanced)</li>
                  </ul>
                </div>
              </div>
            </div>
          )}

          <div className="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-900">
            <label
              htmlFor="confirm-input"
              className="block text-sm font-medium text-gray-900 dark:text-white"
            >
              Type <span className="font-mono font-bold">CONFIRM</span> to proceed
            </label>
            <input
              id="confirm-input"
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value.toUpperCase())}
              placeholder="Type CONFIRM"
              className="mt-2 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-mono text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
            />
          </div>
        </div>

        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={handleClose} type="button">
            Cancel
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={!isConfirmed || isPending}
            variant="destructive"
            type="button"
          >
            {isPending ? 'Disabling...' : 'Disable radio'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
