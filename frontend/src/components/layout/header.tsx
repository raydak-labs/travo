import { Sun, Moon, Menu } from 'lucide-react';
import { useTheme } from './use-theme';
import { useSystemInfo } from '@/hooks/use-system';
import { Button } from '@/components/ui/button';
import { HeaderNotificationsMenu } from './header-notifications-menu';
import { HeaderOverflowMenu } from './header-overflow-menu';
import { HeaderRouterStatus } from './header-router-status';

interface HeaderProps {
  title: string;
  showMenuButton?: boolean;
  onMenuToggle?: () => void;
}

export function Header({ title, showMenuButton, onMenuToggle }: HeaderProps) {
  const { theme, toggleTheme } = useTheme();
  const { data: systemInfo, isError: systemError } = useSystemInfo();

  return (
    <header className="flex h-14 items-center justify-between border-b border-gray-200 bg-white px-4 theme-transition sm:px-6 dark:border-white/10 dark:bg-gray-950">
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
        <HeaderRouterStatus systemInfo={systemInfo} systemError={systemError} />
        <HeaderNotificationsMenu />
        <HeaderOverflowMenu />
        <Button variant="ghost" size="sm" onClick={toggleTheme} aria-label="Toggle theme">
          {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </Button>
      </div>
    </header>
  );
}
