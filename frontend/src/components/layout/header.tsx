import { useState, useRef, useEffect } from 'react';
import { Sun, Moon, LogOut, Menu, Bell, RotateCcw, MoreVertical, PowerOff } from 'lucide-react';
import { useTheme } from './theme-provider';
import { useAuthStore } from '@/stores/auth-store';
import { useAlerts } from '@/hooks/use-alerts';
import { useSystemInfo, useReboot, useShutdown } from '@/hooks/use-system';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';

interface HeaderProps {
  title: string;
  /** Show the hamburger menu button (mobile) */
  showMenuButton?: boolean;
  /** Called when the hamburger button is clicked */
  onMenuToggle?: () => void;
}

const severityVariant: Record<string, 'default' | 'warning' | 'destructive'> = {
  info: 'default',
  warning: 'warning',
  critical: 'destructive',
};

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function Header({ title, showMenuButton, onMenuToggle }: HeaderProps) {
  const { theme, toggleTheme } = useTheme();
  const logout = useAuthStore((s) => s.logout);
  const { alerts, unreadCount, markAllRead } = useAlerts();
  const { data: systemInfo, isError: systemError } = useSystemInfo();
  const rebootMutation = useReboot();
  const shutdownMutation = useShutdown();
  const [showPanel, setShowPanel] = useState(false);
  const [showActionsMenu, setShowActionsMenu] = useState(false);
  const [showRebootConfirm, setShowRebootConfirm] = useState(false);
  const [showShutdownConfirm, setShowShutdownConfirm] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const actionsRef = useRef<HTMLDivElement>(null);

  const isConnected = !!systemInfo && !systemError;

  // Close panel on click outside
  useEffect(() => {
    if (!showPanel && !showActionsMenu) return;
    function handleClick(e: MouseEvent) {
      if (showPanel && panelRef.current && !panelRef.current.contains(e.target as Node)) {
        setShowPanel(false);
      }
      if (showActionsMenu && actionsRef.current && !actionsRef.current.contains(e.target as Node)) {
        setShowActionsMenu(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showPanel, showActionsMenu]);

  return (
    <header className="flex h-14 items-center justify-between border-b border-gray-200 bg-white px-4 theme-transition sm:px-6 dark:border-gray-800 dark:bg-gray-950">
      <div className="flex items-center gap-3">
        {showMenuButton && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onMenuToggle}
            aria-label="Open menu"
            className="-ml-1"
          >
            <Menu className="h-5 w-5" />
          </Button>
        )}
        <h1 className="text-lg font-semibold text-gray-900 dark:text-white">{title}</h1>
      </div>
      <div className="flex items-center gap-1 sm:gap-2">
        {/* Router hostname */}
        {systemInfo?.hostname && (
          <span className="hidden text-xs text-gray-500 sm:block dark:text-gray-400">
            {systemInfo.hostname}
          </span>
        )}

        {/* Connection status indicator */}
        <span
          className={`inline-block h-2 w-2 rounded-full ${
            isConnected
              ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.6)]'
              : 'bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.6)]'
          }`}
          title={isConnected ? `Connected to ${systemInfo?.hostname ?? 'router'}` : 'Connection lost'}
        />

        {/* Notification bell */}
        <div className="relative" ref={panelRef}>
          <Button
            variant="ghost"
            size="sm"
            aria-label="Notifications"
            onClick={() => {
              setShowPanel((v) => !v);
              if (!showPanel) markAllRead();
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
            <div className="absolute right-0 top-full z-50 mt-1 w-80 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900">
              <div className="border-b border-gray-200 px-3 py-2 dark:border-gray-700">
                <span className="text-sm font-semibold text-gray-900 dark:text-white">
                  Notifications
                </span>
              </div>
              <div className="max-h-72 overflow-y-auto">
                {alerts.length === 0 ? (
                  <div className="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                    No notifications
                  </div>
                ) : (
                  alerts.slice(0, 20).map((alert) => (
                    <div
                      key={alert.id}
                      className="flex items-start gap-2 border-b border-gray-100 px-3 py-2 last:border-b-0 dark:border-gray-800"
                    >
                      <Badge
                        variant={severityVariant[alert.severity] ?? 'default'}
                        className="mt-0.5 shrink-0 text-[10px]"
                      >
                        {alert.severity}
                      </Badge>
                      <div className="min-w-0 flex-1">
                        <p className="text-sm text-gray-800 dark:text-gray-200">{alert.message}</p>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          {formatTime(alert.timestamp)}
                        </p>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        {/* Actions menu */}
        <div className="relative" ref={actionsRef}>
          <Button
            variant="ghost"
            size="sm"
            aria-label="Actions"
            onClick={() => setShowActionsMenu((v) => !v)}
          >
            <MoreVertical className="h-4 w-4" />
          </Button>

          {showActionsMenu && (
            <div className="absolute right-0 top-full z-50 mt-1 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
              <button
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                onClick={() => {
                  setShowActionsMenu(false);
                  setShowRebootConfirm(true);
                }}
              >
                <RotateCcw className="h-4 w-4" />
                Reboot Router
              </button>
              <button
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                onClick={() => {
                  setShowActionsMenu(false);
                  setShowShutdownConfirm(true);
                }}
              >
                <PowerOff className="h-4 w-4" />
                Shut Down Router
              </button>
              <button
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                onClick={() => {
                  setShowActionsMenu(false);
                  logout();
                }}
              >
                <LogOut className="h-4 w-4" />
                Logout
              </button>
            </div>
          )}
        </div>

        <Button variant="ghost" size="sm" onClick={toggleTheme} aria-label="Toggle theme">
          {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </Button>
      </div>

      {/* Reboot confirmation dialog */}
      <Dialog open={showRebootConfirm} onOpenChange={setShowRebootConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reboot Router?</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            The router will be unavailable for about 30 seconds during reboot.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowRebootConfirm(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={rebootMutation.isPending}
              onClick={() => {
                rebootMutation.mutate();
                setShowRebootConfirm(false);
              }}
            >
              {rebootMutation.isPending ? 'Rebooting...' : 'Reboot'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Shutdown confirmation dialog */}
      <Dialog open={showShutdownConfirm} onOpenChange={setShowShutdownConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Shut Down Router?</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            The device will power off completely. You will need physical access to turn it back on.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowShutdownConfirm(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={shutdownMutation.isPending}
              onClick={() => {
                shutdownMutation.mutate();
                setShowShutdownConfirm(false);
              }}
            >
              {shutdownMutation.isPending ? 'Shutting down...' : 'Shut Down'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </header>
  );
}
