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
        <div className="flex flex-col gap-3">
          <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
            <div className="text-sm text-amber-800 dark:text-amber-200">
              <p className="font-medium">Important Safety Notice</p>
              <p className="mt-1">
                This change includes automatic rollback protection. If the router becomes unreachable
                after the change, it will automatically revert to the previous configuration within
                30 seconds.
              </p>
            </div>
          </div>
          <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950">
            <p className="text-sm text-blue-800 dark:text-blue-200">
              <span className="font-medium">Stay connected:</span> Keep this page open during the
              switch. If successful, the mode will update automatically. If the rollback occurs,
              your browser will report a connection error — then simply refresh the page to retry.
            </p>
          </div>
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
