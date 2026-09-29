import { useEffect, useRef, useState } from 'react';
import { Bell } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { EmptyState } from '@/components/ui/empty-state';
import { useAlerts } from '@/hooks/use-alerts';
import { headerAlertSeverityVariant } from './header-alert-severity';
import { formatAlertTime } from './header-format-alert-time';
import { usePopoverDismiss, type PopoverCloseReason } from './use-popover-dismiss';

const PANEL_ID = 'header-notifications-panel';

export function HeaderNotificationsMenu() {
  const { alerts, unreadCount, markAllRead } = useAlerts();
  const [showPanel, setShowPanel] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const closeReasonRef = useRef<PopoverCloseReason | null>(null);

  const closePanel = (reason: PopoverCloseReason) => {
    closeReasonRef.current = reason;
    setShowPanel(false);
  };
  const containerRef = usePopoverDismiss<HTMLDivElement>(showPanel, closePanel);

  // Focus the panel on open and return it to the trigger on keyboard
  // dismissal, so the popover is reachable without a pointer.
  useEffect(() => {
    if (showPanel) {
      panelRef.current?.focus();
      return;
    }
    const reason = closeReasonRef.current;
    if (reason === null) return;
    closeReasonRef.current = null;
    if (reason !== 'outside') triggerRef.current?.focus();
  }, [showPanel]);

  return (
    <div className="relative" ref={containerRef}>
      <Button
        ref={triggerRef}
        variant="ghost"
        size="sm"
        aria-label="Notifications"
        aria-expanded={showPanel}
        aria-controls={showPanel ? PANEL_ID : undefined}
        onClick={() => {
          if (showPanel) {
            closePanel('activate');
            return;
          }
          setShowPanel(true);
          markAllRead();
        }}
      >
        <Bell className="h-4 w-4" />
        {unreadCount > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </Button>

      {showPanel && (
        <div
          id={PANEL_ID}
          ref={panelRef}
          role="region"
          aria-label="Notifications"
          tabIndex={-1}
          onKeyDown={(e) => {
            if (e.key === 'Tab') closePanel('tab');
          }}
          className="absolute right-0 top-full z-50 mt-1 w-80 rounded-lg border border-gray-200 bg-white shadow-lg focus:outline-none dark:border-gray-700 dark:bg-gray-900"
        >
          <div className="border-b border-gray-200 px-3 py-2 dark:border-gray-700">
            <span className="text-sm font-semibold text-gray-900 dark:text-white">
              Notifications
            </span>
          </div>
          <div className="max-h-72 overflow-y-auto">
            {alerts.length === 0 ? (
              <EmptyState message="No notifications" />
            ) : (
              alerts.slice(0, 20).map((alert) => (
                <div
                  key={alert.id}
                  className="flex items-start gap-2 border-b border-gray-100 px-3 py-2 last:border-b-0 dark:border-white/[0.08]"
                >
                  <Badge
                    variant={headerAlertSeverityVariant[alert.severity] ?? 'default'}
                    className="mt-0.5 shrink-0 text-[10px]"
                  >
                    {alert.severity}
                  </Badge>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm text-gray-800 dark:text-gray-200">{alert.message}</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {formatAlertTime(alert.timestamp)}
                    </p>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
