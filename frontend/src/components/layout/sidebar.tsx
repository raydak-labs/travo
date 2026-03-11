import { Link, useMatchRoute } from '@tanstack/react-router';
import { cn } from '@/lib/cn';
import {
  Activity,
  Globe,
  Monitor,
  ScrollText,
  Settings,
  Shield,
  Wifi,
  ChevronLeft,
  ChevronRight,
  X,
} from 'lucide-react';

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: Activity },
  { to: '/wifi', label: 'WiFi', icon: Wifi },
  { to: '/network', label: 'Network', icon: Globe },
  { to: '/vpn', label: 'VPN', icon: Shield },
  { to: '/services', label: 'Services', icon: Monitor },
  { to: '/system', label: 'System', icon: Settings },
  { to: '/logs', label: 'Logs', icon: ScrollText },
] as const;

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  /** Called when a navigation link is clicked (used to close mobile drawer) */
  onNavClick?: () => void;
  className?: string;
}

export function Sidebar({ collapsed, onToggle, onNavClick, className }: SidebarProps) {
  const matchRoute = useMatchRoute();
  const inDrawer = !!onNavClick;

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
          <span className="text-sm font-bold text-gray-900 dark:text-white">OpenWRT Travel</span>
        )}
        {inDrawer ? (
          <button
            onClick={onToggle}
            aria-label="Close menu"
            className="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-gray-400 dark:hover:bg-gray-800"
          >
            <X className="h-4 w-4" />
          </button>
        ) : (
          <button
            onClick={onToggle}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            className="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-gray-400 dark:hover:bg-gray-800"
          >
            {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
          </button>
        )}
      </div>

      <nav className="flex-1 space-y-1 p-2" role="navigation">
        {navItems.map(({ to, label, icon: Icon }) => {
          const isActive = !!matchRoute({ to, fuzzy: true });
          return (
            <Link
              key={to}
              to={to}
              onClick={onNavClick}
              className={cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300'
                  : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-white',
                isActive && 'border-l-2 border-blue-600 dark:border-blue-400',
                collapsed && !inDrawer && 'justify-center px-0',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
              )}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {(!collapsed || inDrawer) && <span>{label}</span>}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
