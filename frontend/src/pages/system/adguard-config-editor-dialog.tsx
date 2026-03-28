import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';

type AdGuardConfigEditorDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  configContent: string;
  onConfigContentChange: (value: string) => void;
  onSave: () => void;
  savePending: boolean;
};

export function AdGuardConfigEditorDialog({
  open,
  onOpenChange,
  configContent,
  onConfigContentChange,
  onSave,
  savePending,
}: AdGuardConfigEditorDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>AdGuard Home Configuration</DialogTitle>
          <DialogDescription>
            Edit the AdGuardHome.yaml configuration file. The service will be restarted after
            saving.
          </DialogDescription>
        </DialogHeader>
        <textarea
          className="h-96 w-full rounded-md border border-gray-300 bg-white p-3 font-mono text-sm text-gray-900 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
          value={configContent}
          onChange={(e) => onConfigContentChange(e.target.value)}
          spellCheck={false}
        />
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={onSave} disabled={savePending}>
            {savePending ? 'Saving…' : 'Save & Restart'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
