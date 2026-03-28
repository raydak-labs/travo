import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';

type ApRadioDisableDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isLastActive: boolean;
  onConfirm: () => void;
  confirmPending: boolean;
};

export function ApRadioDisableDialog({
  open,
  onOpenChange,
  isLastActive,
  onConfirm,
  confirmPending,
}: ApRadioDisableDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {isLastActive ? '⚠️ Disable Last Access Point?' : 'Disable Access Point?'}
          </DialogTitle>
        </DialogHeader>
        {isLastActive ? (
          <p className="text-sm text-gray-700 dark:text-gray-300">
            This is the <strong>only active access point</strong>. Disabling it will make the router
            unreachable via WiFi. You will need a wired connection or physical access to re-enable it.
          </p>
        ) : (
          <p className="text-sm text-gray-700 dark:text-gray-300">
            Disabling this access point will disconnect all clients currently connected to it. Are you
            sure?
          </p>
        )}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={confirmPending}>
            Disable
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
