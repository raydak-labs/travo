import { CheckCircle, XCircle, Loader2 } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { useInstallLogStream } from '@/pages/services/use-install-log-stream';

type InstallLogDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  serviceId: string;
  serviceName: string;
  action: 'install' | 'remove';
  onComplete: () => void;
};

export function InstallLogDialog({
  open,
  onOpenChange,
  serviceId,
  serviceName,
  action,
  onComplete,
}: InstallLogDialogProps) {
  const { lines, status, logRef, handleClose } = useInstallLogStream({
    open,
    serviceId,
    action,
    onComplete,
    onOpenChange,
  });

  const title = action === 'install' ? `Installing ${serviceName}` : `Removing ${serviceName}`;

  return (
    <Dialog open={open} onOpenChange={status !== 'streaming' ? handleClose : undefined}>
      <DialogContent
        className="max-w-2xl"
        onInteractOutside={(e) => status === 'streaming' && e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {status === 'streaming' && <Loader2 className="h-4 w-4 animate-spin" />}
            {status === 'done' && <CheckCircle className="h-4 w-4 text-green-500" />}
            {status === 'error' && <XCircle className="h-4 w-4 text-red-500" />}
            {title}
          </DialogTitle>
          <DialogDescription>
            {status === 'streaming' && 'Operation in progress...'}
            {status === 'done' && 'Operation completed successfully.'}
            {status === 'error' && 'Operation failed. Check the log for details.'}
          </DialogDescription>
        </DialogHeader>

        <pre
          ref={logRef}
          className="max-h-80 overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs text-gray-200"
        >
          {lines.length === 0 && status === 'streaming'
            ? 'Waiting for output...\n'
            : lines.join('\n')}
        </pre>

        <DialogFooter>
          <Button
            onClick={handleClose}
            disabled={status === 'streaming'}
            variant={status === 'error' ? 'destructive' : 'default'}
          >
            {status === 'streaming' ? 'Please wait...' : 'Close'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
