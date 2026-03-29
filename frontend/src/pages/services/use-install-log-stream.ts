import { useCallback, useEffect, useRef, useState } from 'react';
import { streamRequest, type StreamEvent } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

export type InstallLogAction = 'install' | 'remove';
export type InstallLogStatus = 'streaming' | 'done' | 'error';

type UseInstallLogStreamOptions = {
  open: boolean;
  serviceId: string;
  action: InstallLogAction;
  onComplete: () => void;
  onOpenChange: (open: boolean) => void;
};

export function useInstallLogStream({
  open,
  serviceId,
  action,
  onComplete,
  onOpenChange,
}: UseInstallLogStreamOptions) {
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState<InstallLogStatus>('streaming');
  const logRef = useRef<HTMLPreElement>(null);
  const startedRef = useRef(false);

  const appendLine = useCallback((line: string) => {
    setLines((prev) => [...prev, line]);
  }, []);

  useEffect(() => {
    if (!open || startedRef.current) return;
    startedRef.current = true;

    const route =
      action === 'install'
        ? API_ROUTES.services.installStream.replace(':id', serviceId)
        : API_ROUTES.services.removeStream.replace(':id', serviceId);

    queueMicrotask(() => {
      setLines([]);
      setStatus('streaming');
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
    });

    return () => {
      startedRef.current = false;
    };
  }, [open, serviceId, action, appendLine]);

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

  return { lines, status, logRef, handleClose };
}
