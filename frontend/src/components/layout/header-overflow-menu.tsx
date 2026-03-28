import { useState, useRef, useEffect } from 'react';
import { LogOut, MoreVertical, RotateCcw, PowerOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { useAuthStore } from '@/stores/auth-store';
import { useReboot, useShutdown } from '@/hooks/use-system';

export function HeaderOverflowMenu() {
  const logout = useAuthStore((s) => s.logout);
  const rebootMutation = useReboot();
  const shutdownMutation = useShutdown();
  const [showMenu, setShowMenu] = useState(false);
  const [showRebootConfirm, setShowRebootConfirm] = useState(false);
  const [showShutdownConfirm, setShowShutdownConfirm] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showMenu) return;
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showMenu]);

  return (
    <>
      <div className="relative" ref={menuRef}>
        <Button
          variant="ghost"
          size="sm"
          aria-label="Actions"
          onClick={() => setShowMenu((v) => !v)}
        >
          <MoreVertical className="h-4 w-4" />
        </Button>

        {showMenu && (
          <div className="absolute right-0 top-full z-50 mt-1 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
              onClick={() => {
                setShowMenu(false);
                setShowRebootConfirm(true);
              }}
            >
              <RotateCcw className="h-4 w-4" />
              Reboot Router
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
              onClick={() => {
                setShowMenu(false);
                setShowShutdownConfirm(true);
              }}
            >
              <PowerOff className="h-4 w-4" />
              Shut Down Router
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
              onClick={() => {
                setShowMenu(false);
                logout();
              }}
            >
              <LogOut className="h-4 w-4" />
              Logout
            </button>
          </div>
        )}
      </div>

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
    </>
  );
}
