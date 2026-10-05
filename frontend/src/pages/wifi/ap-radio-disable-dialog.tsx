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
  /** `save` when triggered by Save rather than by an explicit disable. */
  action?: 'save' | 'disable';
  onConfirm: () => void;
  confirmPending: boolean;
};

export function ApRadioDisableDialog({
  open,
  onOpenChange,
  isLastActive,
  action = 'disable',
  onConfirm,
  confirmPending,
}: ApRadioDisableDialogProps) {
  const isSave = action === 'save';
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {isLastActive
              ? isSave
                ? '⚠️ Turn Off the Last Access Point and Save?'
                : '⚠️ Disable Last Access Point?'
              : isSave
                ? 'Turn Off Access Point and Save?'
                : 'Disable Access Point?'}
          </DialogTitle>
        </DialogHeader>
        {isLastActive ? (
          <p className="text-sm text-gray-700 dark:text-gray-300">
            This is the <strong>only active access point</strong>. Disabling it will make the router
            unreachable via WiFi. You will need a wired connection or physical access to re-enable
            it.
          </p>
        ) : (
          <p className="text-sm text-gray-700 dark:text-gray-300">
            {isSave
              ? 'This turn the access point off and then saved. Clients connected to it are disconnected.'
              : 'Disabling this access point will disconnect all clients currently connected to it. Are you sure?'}
            Are you sure?
          </p>
        )}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={confirmPending}>
            {isSave ? 'Turn Off and Save' : 'Disable'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
