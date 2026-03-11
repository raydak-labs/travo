import { useCallback, useEffect, useRef, useState } from 'react';
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
import { streamRequest, type StreamEvent } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

type Action = 'install' | 'remove';
type Status = 'streaming' | 'done' | 'error';

interface InstallLogDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  serviceId: string;
  serviceName: string;
  action: Action;
  onComplete: () => void;
}

export function InstallLogDialog({
  open,
  onOpenChange,
  serviceId,
  serviceName,
  action,
  onComplete,
}: InstallLogDialogProps) {
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState<Status>('streaming');
  const logRef = useRef<HTMLPreElement>(null);
  const startedRef = useRef(false);

  const appendLine = useCallback((line: string) => {
    setLines((prev) => [...prev, line]);
  }, []);

  useEffect(() => {
    if (!open || startedRef.current) return;
    startedRef.current = true;
    setLines([]);
    setStatus('streaming');

    const route =
      action === 'install'
        ? API_ROUTES.services.installStream.replace(':id', serviceId)
        : API_ROUTES.services.removeStream.replace(':id', serviceId);

    streamRequest(route, (event: StreamEvent) => {
      if (event.type === 'log' && event.data) {
        appendLine(event.data);
      } else if (event.type === 'done') {
        setStatus('done');
      } else if (event.type === 'error') {
        appendLine(`ERROR: ${event.data ?? 'Unknown error'}`);
        setStatus('error');
      }
    }).catch((err: Error) => {
      appendLine(`ERROR: ${err.message}`);
      setStatus('error');
    });

    return () => {
      startedRef.current = false;
    };
  }, [open, serviceId, action, appendLine]);

  // Auto-scroll to bottom when new lines arrive
  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [lines]);

  const handleClose = () => {
    if (status !== 'streaming') {
      onComplete();
      onOpenChange(false);
      startedRef.current = false;
    }
  };

  const title = action === 'install' ? `Installing ${serviceName}` : `Removing ${serviceName}`;

  return (
    <Dialog open={open} onOpenChange={status !== 'streaming' ? handleClose : undefined}>
      <DialogContent className="max-w-2xl" onInteractOutside={(e) => status === 'streaming' && e.preventDefault()}>
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
          {lines.length === 0 && status === 'streaming' ? 'Waiting for output...\n' : lines.join('\n')}
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
