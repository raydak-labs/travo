import { useState } from 'react';
import { Monitor, Radio, Smartphone } from 'lucide-react';
import type { WifiMode } from '@shared/index';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { clsx } from 'clsx';

interface WifiModeSelectorProps {
  currentMode: WifiMode;
  onModeChange: (mode: WifiMode) => void;
  isChanging: boolean;
}

const modes: Array<{ mode: WifiMode; icon: typeof Monitor; title: string; description: string }> = [
  {
    mode: 'client',
    icon: Smartphone,
    title: 'Client',
    description: 'Connect to an existing WiFi network as a client device',
  },
  {
    mode: 'repeater',
    icon: Radio,
    title: 'Repeater',
    description: 'Extend an existing WiFi network and rebroadcast the signal',
  },
  {
    mode: 'ap',
    icon: Monitor,
    title: 'Access Point',
    description: 'Create a new WiFi network for other devices to connect to',
  },
];

export function WifiModeSelector({ currentMode, onModeChange, isChanging }: WifiModeSelectorProps) {
  const [pendingMode, setPendingMode] = useState<WifiMode | null>(null);

  function handleSelect(mode: WifiMode) {
    if (mode === currentMode) return;
    setPendingMode(mode);
  }

  function handleConfirm() {
    if (pendingMode) {
      onModeChange(pendingMode);
      setPendingMode(null);
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white">WiFi Mode</h3>
      <div className="grid gap-3 sm:grid-cols-3">
        {modes.map(({ mode, icon: Icon, title, description }) => (
          <Card
            key={mode}
            className={clsx(
              'cursor-pointer transition-colors',
              mode === currentMode
                ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-950'
                : 'hover:border-gray-300 dark:hover:border-gray-600',
            )}
            onClick={() => handleSelect(mode)}
            role="button"
            aria-pressed={mode === currentMode}
            aria-label={`${title} mode`}
          >
            <CardContent className="flex flex-col items-center p-4 text-center">
              <Icon
                className={clsx(
                  'mb-2 h-8 w-8',
                  mode === currentMode ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400',
                )}
              />
              <p className="text-sm font-medium text-gray-900 dark:text-white">{title}</p>
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">{description}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {pendingMode && (
        <div className="flex items-center gap-2 rounded-lg border border-yellow-300 bg-yellow-50 p-3 dark:border-yellow-700 dark:bg-yellow-950">
          <p className="flex-1 text-sm text-yellow-800 dark:text-yellow-200">
            Switch to <strong>{pendingMode}</strong> mode? This will temporarily disconnect WiFi.
          </p>
          <Button size="sm" onClick={handleConfirm} disabled={isChanging}>
            {isChanging ? 'Switching...' : 'Confirm'}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setPendingMode(null)}
            disabled={isChanging}
          >
            Cancel
          </Button>
        </div>
      )}
    </div>
  );
}
