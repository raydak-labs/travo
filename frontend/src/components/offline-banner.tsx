import { WifiOff } from 'lucide-react';
import { useRouterReachable } from '@/hooks/use-router-reachable';

export function OfflineBanner() {
  // Router reachability, not `navigator.onLine`: see use-router-reachable.
  const routerReachable = useRouterReachable();

  if (routerReachable) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex items-center justify-center gap-2 bg-[var(--status-warn-surface)] px-4 py-2 text-sm font-medium text-[var(--status-warn-text)]"
    >
      <WifiOff className="h-4 w-4" aria-hidden="true" />
      <span>
        Cannot reach the router. Your device may still have internet, but this page needs the
        router.
      </span>
    </div>
  );
}
