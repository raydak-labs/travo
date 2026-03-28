import { Link, useRouterState } from '@tanstack/react-router';
import { cn } from '@/lib/cn';
import { NAV_ENTRIES, flattenNavRoutes, isRouteActive } from '@/components/layout/nav-config';
import { SidebarNavGroup } from '@/components/layout/sidebar-nav-group';
import { useSidebarGroups } from '@/components/layout/use-sidebar-groups';
import { ChevronLeft, ChevronRight, X } from 'lucide-react';

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  /** Called when a navigation link is clicked (used to close mobile drawer) */
  onNavClick?: () => void;
  className?: string;
}

export function Sidebar({ collapsed, onToggle, onNavClick, className }: SidebarProps) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const inDrawer = !!onNavClick;
  const { groupOpen, setGroup } = useSidebarGroups(pathname);

  const linkClass = (active: boolean, rail: boolean) =>
    cn(
      'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
      active
        ? 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300'
        : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-white',
      active && 'border-l-2 border-blue-600 dark:border-blue-400',
      rail && 'justify-center px-0',
    );

  const collapsedRail = collapsed && !inDrawer;
  const flat = flattenNavRoutes();

  return (
    <aside
      className={cn(
        'flex h-full flex-col border-r border-gray-200 bg-white theme-transition dark:border-gray-800 dark:bg-gray-950',
        !inDrawer && 'transition-all duration-200',
        !inDrawer && (collapsed ? 'w-16' : 'w-56'),
        className,
      )}
    >
      <div className="flex h-14 items-center justify-between border-b border-gray-200 px-3 dark:border-gray-800">
        {(!collapsed || inDrawer) && (
          <span className="truncate text-sm font-bold text-gray-900 dark:text-white">
            OpenWRT Travel
          </span>
        )}
        {inDrawer ? (
          <button
            type="button"
            onClick={onToggle}
            aria-label="Close menu"
            className="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-gray-400 dark:hover:bg-gray-800"
          >
            <X className="h-4 w-4" />
          </button>
        ) : (
          <button
            type="button"
            onClick={onToggle}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            className="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-gray-400 dark:hover:bg-gray-800"
          >
            {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
          </button>
        )}
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto p-2" role="navigation" aria-label="Main">
        {collapsedRail ? (
          <>
            {flat.map(({ to, label, icon: Icon }) => {
              const active = isRouteActive(to, pathname);
              return (
                <Link
                  key={to}
                  to={to}
                  title={label}
                  onClick={onNavClick}
                  className={linkClass(active, true)}
                >
                  <Icon className="h-5 w-5 shrink-0" aria-hidden />
                </Link>
              );
            })}
          </>
        ) : (
          <>
            {NAV_ENTRIES.map((entry) => {
              if (entry.kind === 'leaf') {
                const active = isRouteActive(entry.to, pathname);
                const Icon = entry.icon;
                return (
                  <Link
                    key={entry.id}
                    to={entry.to}
                    onClick={onNavClick}
                    className={linkClass(active, false)}
                  >
                    <Icon className="h-5 w-5 shrink-0" aria-hidden />
                    <span>{entry.label}</span>
                  </Link>
                );
              }

              return (
                <SidebarNavGroup
                  key={entry.id}
                  group={entry}
                  pathname={pathname}
                  open={groupOpen[entry.id] ?? true}
                  onOpenChange={(o) => setGroup(entry.id, o)}
                  onNavClick={onNavClick}
                />
              );
            })}
          </>
        )}
      </nav>
    </aside>
  );
}
