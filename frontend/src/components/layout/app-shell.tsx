import { useState, type ReactNode } from 'react';
import { Sidebar } from './sidebar';
import { Header } from './header';
import { Sheet, SheetContent } from '@/components/ui/sheet';
import { useIsMobile } from '@/hooks/use-mobile';
import { useSessionTimeout } from '@/hooks/use-session-timeout';

interface AppShellProps {
  children: ReactNode;
  title: string;
}

export function AppShell({ children, title }: AppShellProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const isMobile = useIsMobile();
  useSessionTimeout();

  return (
    <div className="flex h-screen overflow-hidden bg-gray-50 theme-transition dark:bg-gray-900">
      {/* Desktop sidebar */}
      {!isMobile && <Sidebar collapsed={collapsed} onToggle={() => setCollapsed((c) => !c)} />}

      {/* Mobile drawer */}
      {isMobile && (
        <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
          <SheetContent side="left" className="w-72 p-0">
            <Sidebar
              collapsed={false}
              onToggle={() => setMobileOpen(false)}
              onNavClick={() => setMobileOpen(false)}
              className="w-full border-r-0"
            />
          </SheetContent>
        </Sheet>
      )}

      <div className="flex flex-1 flex-col overflow-hidden">
        <Header title={title} showMenuButton={isMobile} onMenuToggle={() => setMobileOpen(true)} />
        <main className="flex-1 overflow-y-auto p-4 sm:p-6">
          <div className="animate-page-fade-in">{children}</div>
        </main>
      </div>
    </div>
  );
}
