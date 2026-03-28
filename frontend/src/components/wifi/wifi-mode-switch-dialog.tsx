import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

interface WifiModeSwitchDialogProps {
  open: boolean;
  modeLabel: string | undefined;
  isPending: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export function WifiModeSwitchDialog({
  open,
  modeLabel,
  isPending,
  onOpenChange,
  onConfirm,
}: WifiModeSwitchDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Switch WiFi Mode</DialogTitle>
          <DialogDescription>
            Are you sure you want to switch to{' '}
            <span className="font-medium text-gray-900 dark:text-white">{modeLabel}</span> mode?
          </DialogDescription>
        </DialogHeader>
        <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
          <p className="text-sm text-amber-800 dark:text-amber-200">
            Changing the WiFi mode will restart the wireless subsystem. You may temporarily lose
            connectivity and need to reconnect to the router.
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={onConfirm} disabled={isPending}>
            {isPending ? 'Switching...' : 'Switch Mode'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
