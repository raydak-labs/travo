import { useEffect, useMemo, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';

interface OperationProgressDialogProps {
  open: boolean;
  title: string;
  description?: string;
  details?: string[];
}

export function OperationProgressDialog({ open, title, description, details }: OperationProgressDialogProps) {
  const [startedAt] = useState(() => (open ? Date.now() : 0));
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    if (!open) return;
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, [open]);

  const elapsedSeconds = useMemo(() => {
    if (!open || startedAt === 0) return 0;
    return Math.max(0, Math.floor((now - startedAt) / 1000));
  }, [now, open, startedAt]);

  return (
    <Dialog open={open}>
      <DialogContent className="sm:max-w-md" onPointerDownOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            {title}
          </DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>

        <div className="space-y-3 text-sm">
          <div className="text-xs text-gray-500 dark:text-gray-400">Elapsed: {elapsedSeconds}s</div>
          {details && details.length > 0 && (
            <ul className="space-y-1.5 rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-gray-900 dark:text-gray-300">
              {details.map((d) => (
                <li key={d}>{d}</li>
              ))}
            </ul>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

