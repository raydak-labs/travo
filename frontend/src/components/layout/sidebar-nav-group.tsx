import { Link } from '@tanstack/react-router';
import { cn } from '@/lib/cn';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { ChevronDown } from 'lucide-react';
import type { NavGroup } from '@/components/layout/nav-config';
import { isRouteActive } from '@/components/layout/nav-config';

function subLinkClass(active: boolean) {
  return cn(
    'flex w-full items-center rounded-md py-2 pl-9 pr-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
    active
      ? 'bg-blue-50 font-medium text-blue-700 dark:bg-blue-950 dark:text-blue-300'
      : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-white',
  );
}

export function SidebarNavGroup({
  group,
  pathname,
  open,
  onOpenChange,
  onNavClick,
}: {
  group: NavGroup;
  pathname: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onNavClick?: () => void;
}) {
  const GroupIcon = group.icon;
  const groupActive = group.items.some((item) => isRouteActive(item.to, pathname));
  const defaultActive = isRouteActive(group.defaultTo, pathname);
  const submenuItems = group.items.filter((item) => item.to !== group.defaultTo);

  return (
    <Collapsible open={open} onOpenChange={onOpenChange}>
      <div
        className={cn(
          'flex items-center gap-0.5 rounded-md',
          groupActive && !defaultActive && 'bg-gray-100 dark:bg-gray-800',
        )}
      >
        <Link
          to={group.defaultTo}
          onClick={onNavClick}
          activeOptions={{ exact: true }}
          className={cn(
            'flex min-w-0 flex-1 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
            defaultActive
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300'
              : groupActive
                ? 'text-gray-900 dark:text-white'
                : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800',
          )}
        >
          <GroupIcon className="h-5 w-5 shrink-0 text-gray-500 dark:text-gray-400" aria-hidden />
          <span className="truncate">{group.label}</span>
        </Link>
        <CollapsibleTrigger
          type="button"
          aria-label={`Toggle ${group.label} menu`}
          className={cn(
            'flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors',
            'hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
            'dark:text-gray-400 dark:hover:bg-gray-800',
          )}
        >
          <ChevronDown
            className={cn('h-4 w-4 transition-transform', open && 'rotate-180')}
            aria-hidden
          />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent>
        <ul className="mt-1 space-y-0.5 border-l border-gray-200 pl-2 dark:border-gray-700">
          {submenuItems.map((item) => {
            const active = isRouteActive(item.to, pathname);
            return (
              <li key={item.to}>
                <Link
                  to={item.to}
                  onClick={onNavClick}
                  className={subLinkClass(active)}
                  activeOptions={{ exact: true }}
                >
                  {item.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  );
}
