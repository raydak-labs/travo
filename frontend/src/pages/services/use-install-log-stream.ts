import { useCallback, useEffect, useRef, useState } from 'react';
import { streamRequest, type StreamEvent } from '@/lib/api-client';
import { routeWithParam } from '@/lib/api-url';
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
  const abortRef = useRef<AbortController | null>(null);

  const abortStream = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
  }, []);

  const appendLine = useCallback((line: string) => {
    setLines((prev) => [...prev, line]);
  }, []);

  useEffect(() => {
    if (!open || startedRef.current) return;
    startedRef.current = true;

    const controller = new AbortController();
    abortRef.current = controller;

    const route =
      action === 'install'
        ? routeWithParam(API_ROUTES.services.installStream, serviceId)
        : routeWithParam(API_ROUTES.services.removeStream, serviceId);

    queueMicrotask(() => {
      if (controller.signal.aborted) return;
      setLines([]);
      setStatus('streaming');
      streamRequest(
        route,
        (event: StreamEvent) => {
          if (event.type === 'log' && event.data) {
            appendLine(event.data);
          } else if (event.type === 'done') {
            setStatus('done');
          } else if (event.type === 'error') {
            appendLine(`ERROR: ${event.data ?? 'Unknown error'}`);
            setStatus('error');
          }
        },
        controller.signal,
      ).catch((err: Error) => {
        if (controller.signal.aborted) return;
        appendLine(`ERROR: ${err.message}`);
        setStatus('error');
      });
    });

    return () => {
      startedRef.current = false;
      // Without the abort the router keeps streaming into `lines` after the
      // dialog closed, and reopening it interleaves a second stream into the
      // same array.
      abortStream();
    };
  }, [open, serviceId, action, appendLine, abortStream]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [lines]);

  const handleClose = () => {
    if (status !== 'streaming') {
      onComplete();
      onOpenChange(false);
      abortStream();
      startedRef.current = false;
    }
  };

  return { lines, status, logRef, handleClose };
}
