import { useState } from 'react';
import { AlertTriangle, Radio } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
  /** Total radios on the device. `SetRadioRole` only touches the named radio. */
  radioCount: number;
  isPending: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export function ConfirmRadioDisableDialog({
  open,
  radioName,
  radioCount,
  isPending,
  onOpenChange,
  onConfirm,
}: ConfirmRadioDisableDialogProps) {
  const { data: connectionMethod } = useConnectionMethod();
  const isWifiClient = connectionMethod?.method === 'wifi-client';
  const isOnlyRadio = radioCount <= 1;

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
          <DialogTitle className="flex items-center gap-2 text-[var(--status-danger-text)]">
            <AlertTriangle className="h-5 w-5" />
            Disable WiFi Radio
          </DialogTitle>
          <DialogDescription>
            You are about to disable the radio{' '}
            <span className="font-medium text-gray-900 dark:text-white">{radioName}</span>.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="rounded-lg border border-[var(--status-danger-border)] bg-[var(--status-danger-surface)] p-4">
            <div className="flex items-start gap-3">
              <Radio className="mt-0.5 h-5 w-5 shrink-0 text-[var(--status-danger-text)]" />
              <div className="flex-1">
                {/* `SetRadioRole` disables only the sections bound to the
                    named radio. Claiming "all WiFi" on a two-radio device is a
                    false all-clear about blast radius in the one dialog that is
                    supposed to be maximally scary. */}
                <h3 className="text-sm font-semibold text-[var(--status-danger-text)]">
                  {isOnlyRadio
                    ? 'All WiFi will stop working'
                    : `This radio will stop working — the other band stays up`}
                </h3>
                <p className="mt-1 text-sm text-[var(--status-danger-text)]">
                  Disabling {radioName} turns off everything running on it:
                </p>
                <ul className="mt-2 space-y-1 text-sm text-[var(--status-danger-text)]">
                  <li>• WiFi client connections (uplink) on {radioName}</li>
                  <li>• Access points and guest networks hosted on {radioName}</li>
                  <li>• Devices currently connected to those networks</li>
                </ul>
                {!isOnlyRadio && (
                  <p className="mt-2 text-sm text-[var(--status-danger-text)]">
                    The other radio keeps working, so devices on that band stay connected.
                  </p>
                )}
              </div>
            </div>
          </div>

          {isWifiClient && (
            <div className="rounded-lg border-2 border-[var(--status-danger-border)] bg-[var(--status-danger-surface)] p-4">
              <div className="flex items-start gap-3">
                <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-[var(--status-danger-text)]" />
                <div>
                  <h3 className="text-sm font-bold text-[var(--status-danger-text)]">
                    You will lose all access to this device
                  </h3>
                  <p className="mt-1 text-sm text-[var(--status-danger-text)]">
                    You are currently connected via WiFi client. Disabling the radio will cut off
                    your connection immediately.
                  </p>
                  <p className="mt-2 text-sm text-[var(--status-danger-text)]">
                    <strong>Recovery options:</strong>
                  </p>
                  <ul className="mt-1 space-y-1 text-sm text-[var(--status-danger-text)]">
                    <li>• Connect via Ethernet cable to LAN port</li>
                    <li>• Reboot the router and connect to the AP network it comes up with</li>
                    <li>• Access via serial console (advanced)</li>
                  </ul>
                </div>
              </div>
            </div>
          )}

          <div className="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-900">
            <Label
              htmlFor="confirm-input"
              className="block text-sm font-medium text-gray-900 dark:text-white"
            >
              Type <span className="font-mono font-bold">CONFIRM</span> to proceed
            </Label>
            <Input
              id="confirm-input"
              className="mt-2 font-mono"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value.toUpperCase())}
              placeholder="Type CONFIRM"
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
