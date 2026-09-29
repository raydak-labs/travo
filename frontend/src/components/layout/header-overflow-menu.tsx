import { useEffect, useRef, useState } from 'react';
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
import { moveMenuFocus, usePopoverDismiss, type PopoverCloseReason } from './use-popover-dismiss';

const MENU_ID = 'header-overflow-menu';

export function HeaderOverflowMenu() {
  const logout = useAuthStore((s) => s.logout);
  const rebootMutation = useReboot();
  const shutdownMutation = useShutdown();
  const [showMenu, setShowMenu] = useState(false);
  const [showRebootConfirm, setShowRebootConfirm] = useState(false);
  const [showShutdownConfirm, setShowShutdownConfirm] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const closeReasonRef = useRef<PopoverCloseReason | null>(null);

  const closeMenu = (reason: PopoverCloseReason) => {
    closeReasonRef.current = reason;
    setShowMenu(false);
  };
  const containerRef = usePopoverDismiss<HTMLDivElement>(showMenu, closeMenu);

  // Move focus into the menu when it opens, and back to the trigger when it
  // closes for any reason other than an outside pointer click.
  useEffect(() => {
    if (showMenu) {
      containerRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
      return;
    }
    const reason = closeReasonRef.current;
    if (reason === null) return;
    closeReasonRef.current = null;
    if (reason !== 'outside') triggerRef.current?.focus();
  }, [showMenu, containerRef]);

  const openConfirm = (which: 'reboot' | 'shutdown') => {
    closeMenu('activate');
    if (which === 'reboot') setShowRebootConfirm(true);
    else setShowShutdownConfirm(true);
  };

  const itemClass =
    'flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 focus:bg-gray-100 focus:outline-none dark:text-gray-300 dark:hover:bg-gray-800 dark:focus:bg-gray-800';

  return (
    <>
      <div className="relative" ref={containerRef}>
        <Button
          ref={triggerRef}
          variant="ghost"
          size="sm"
          aria-label="Actions"
          aria-haspopup="menu"
          aria-expanded={showMenu}
          aria-controls={showMenu ? MENU_ID : undefined}
          onClick={() => (showMenu ? closeMenu('activate') : setShowMenu(true))}
        >
          <MoreVertical className="h-4 w-4" />
        </Button>

        {showMenu && (
          <div
            id={MENU_ID}
            role="menu"
            aria-label="Router actions"
            className="absolute right-0 top-full z-50 mt-1 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
            onKeyDown={(e) => {
              if (e.key === 'Tab') {
                closeMenu('tab');
                return;
              }
              if (moveMenuFocus(containerRef.current, e.key)) e.preventDefault();
            }}
          >
            <button
              type="button"
              role="menuitem"
              tabIndex={-1}
              className={itemClass}
              onClick={() => openConfirm('reboot')}
            >
              <RotateCcw className="h-4 w-4" />
              Reboot Router
            </button>
            <button
              type="button"
              role="menuitem"
              tabIndex={-1}
              className={itemClass}
              onClick={() => openConfirm('shutdown')}
            >
              <PowerOff className="h-4 w-4" />
              Shut Down Router
            </button>
            <button
              type="button"
              role="menuitem"
              tabIndex={-1}
              className={itemClass}
              onClick={() => {
                closeMenu('activate');
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
                // Stay open while the mutation runs so the pending state is not
                // rendered into an unmounted dialog.
                rebootMutation.mutate(undefined, {
                  onSettled: () => setShowRebootConfirm(false),
                });
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
                // Stay open while the mutation runs so the pending state is not
                // rendered into an unmounted dialog.
                shutdownMutation.mutate(undefined, {
                  onSettled: () => setShowShutdownConfirm(false),
                });
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
