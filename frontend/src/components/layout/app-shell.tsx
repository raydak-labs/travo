import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouterState } from '@tanstack/react-router';
import { Sidebar } from './sidebar';
import { useSidebarCollapsed } from './use-sidebar-collapsed';
import { Header } from './header';
import { Sheet, SheetContent } from '@/components/ui/sheet';
import { useIsMobile } from '@/hooks/use-mobile';
import { useSessionTimeout } from '@/hooks/use-session-timeout';
import { OfflineBanner } from '@/components/offline-banner';

interface AppShellProps {
  children: ReactNode;
  title: string;
}

export function AppShell({ children, title }: AppShellProps) {
  const [collapsed, setCollapsed] = useSidebarCollapsed();
  const isMobile = useIsMobile();
  const mainRef = useRef<HTMLElement>(null);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  useSessionTimeout();

  // Derived rather than an effect: the drawer is open only for the route it
  // was opened on, so a navigation that does not originate from a drawer link
  // (a redirect, a dialog that navigates) closes it, with no cascading render.
  const [drawerOpenedOn, setDrawerOpenedOn] = useState<string | null>(null);
  const mobileOpen = drawerOpenedOn === pathname;

  // SPA navigation does not move focus, so the new page was never announced
  // and keyboard users stayed parked on the sidebar link they clicked.
  useEffect(() => {
    mainRef.current?.focus();
  }, [pathname]);

  return (
    // `h-dvh`, not `h-screen`: 100vh exceeds the visible area on mobile once
    // browser chrome collapses, and combined with overflow-hidden that made
    // the bottom of a long form — including its submit button — unreachable.
    <div className="flex h-dvh overflow-hidden bg-gray-50 theme-transition dark:bg-gray-900">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:shadow-lg focus:ring-2 focus:ring-blue-500 dark:focus:bg-gray-800"
      >
        Skip to main content
      </a>

      {/* Desktop sidebar */}
      {!isMobile && <Sidebar collapsed={collapsed} onToggle={() => setCollapsed((c) => !c)} />}

      {/* Mobile drawer */}
      {isMobile && (
        <Sheet
          open={mobileOpen}
          onOpenChange={(open) => setDrawerOpenedOn(open ? pathname : null)}
        >
          <SheetContent side="left" className="w-72 p-0">
            <Sidebar
              collapsed={false}
              onToggle={() => setDrawerOpenedOn(null)}
              onNavClick={() => setDrawerOpenedOn(null)}
              className="w-full border-r-0"
            />
          </SheetContent>
        </Sheet>
      )}

      <div className="flex flex-1 flex-col overflow-hidden">
        <Header title={title} showMenuButton={isMobile} onMenuToggle={() => setDrawerOpenedOn(pathname)} />
        <OfflineBanner />
        <main id="main-content" tabIndex={-1} ref={mainRef} className="flex-1 overflow-y-auto p-4 sm:p-6 focus:outline-none">
          <div className="animate-page-fade-in">{children}</div>
        </main>
      </div>
    </div>
  );
}
