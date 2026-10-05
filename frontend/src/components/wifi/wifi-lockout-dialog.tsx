import { useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

type WifiLockoutDialogProps = {
  open: boolean;
  isPending: boolean;
  /** Dismiss: nothing is re-sent, and the change stays unapplied. */
  onCancel: () => void;
  /** The operator ticked the box: re-send the same request with the ack. */
  onConfirm: () => void;
};

/**
 * The acknowledgement half of the wireless lockout guard (ADR 0002 §5).
 *
 * It is a checkbox, not a dismissible confirmation, on purpose: the request
 * behind it has already been refused by the router, and the one thing that
 * makes the next attempt succeed is the operator saying out loud that they know
 * they are about to cut their own connection. A dialog that could be accepted
 * with a stray click would reintroduce exactly the lockout this exists to stop.
 */
export function WifiLockoutDialog({
  open,
  isPending,
  onCancel,
  onConfirm,
}: WifiLockoutDialogProps) {
  function handleOpenChange(next: boolean) {
    if (!next) onCancel();
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {/* Keyed on `open` so the checkbox state dies with every close: a tick
          made for one refused request must not still be there for the next
          one. */}
      <WifiLockoutContent
        key={open ? 'lockout-open' : 'lockout-closed'}
        isPending={isPending}
        onCancel={() => handleOpenChange(false)}
        onConfirm={onConfirm}
      />
    </Dialog>
  );
}

function WifiLockoutContent({
  isPending,
  onCancel,
  onConfirm,
}: Omit<WifiLockoutDialogProps, 'open'>) {
  const [acknowledged, setAcknowledged] = useState(false);

  return (
    <>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-red-600 dark:text-red-400">
            <AlertTriangle className="h-5 w-5" />
            This will disconnect you
          </DialogTitle>
          <DialogDescription>
            You are connected over WiFi, and this change would remove the access point you are
            using. Nothing has been changed yet.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="rounded-lg border border-red-300 bg-red-50 p-4 dark:border-red-700 dark:bg-red-950">
            <p className="text-sm text-red-900 dark:text-red-100">
              Your device will lose its connection to this router and will not be able to rejoin.
            </p>
            <p className="mt-2 text-sm text-red-800 dark:text-red-200">
              <strong>Before you continue:</strong> connect this device to the router with an
              Ethernet cable. If you have no cable, do not continue — you will need physical access
              to the router to turn WiFi back on.
            </p>
          </div>

          <div className="flex items-start gap-3 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-900">
            <input
              type="checkbox"
              id="wifi-lockout-acknowledge"
              className="mt-0.5 h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800"
              checked={acknowledged}
              onChange={(event) => setAcknowledged(event.target.checked)}
            />
            <Label
              htmlFor="wifi-lockout-acknowledge"
              className="text-sm font-medium text-gray-900 dark:text-white"
            >
              I am not connected over WiFi, or I accept that this will disconnect me and leave me
              without a way back in.
            </Label>
          </div>
        </div>

        <DialogFooter className="gap-2">
          <Button variant="outline" type="button" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            type="button"
            disabled={!acknowledged || isPending}
            onClick={onConfirm}
          >
            {isPending ? 'Applying…' : 'Apply anyway'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </>
  );
}
