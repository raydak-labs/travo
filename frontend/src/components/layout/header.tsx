import { Sun, Moon, LogOut, Menu } from 'lucide-react';
import { useTheme } from './theme-provider';
import { useAuthStore } from '@/stores/auth-store';
import { Button } from '@/components/ui/button';

interface HeaderProps {
  title: string;
  /** Show the hamburger menu button (mobile) */
  showMenuButton?: boolean;
  /** Called when the hamburger button is clicked */
  onMenuToggle?: () => void;
}

export function Header({ title, showMenuButton, onMenuToggle }: HeaderProps) {
  const { theme, toggleTheme } = useTheme();
  const logout = useAuthStore((s) => s.logout);

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
        <Button variant="ghost" size="sm" onClick={toggleTheme} aria-label="Toggle theme">
          {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </Button>
        <Button variant="ghost" size="sm" onClick={logout} aria-label="Logout">
          <LogOut className="h-4 w-4" />
        </Button>
      </div>
    </header>
  );
}
