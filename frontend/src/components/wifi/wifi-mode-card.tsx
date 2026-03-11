import { useState } from 'react';
import { Wifi, Monitor, Repeat, AlertTriangle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { useWifiConnection, useWifiMode } from '@/hooks/use-wifi';
import { cn } from '@/lib/cn';
import type { WifiMode } from '@shared/index';

interface ModeOption {
  mode: WifiMode;
  label: string;
  icon: typeof Wifi;
  description: string;
}

const MODE_OPTIONS: ModeOption[] = [
  {
    mode: 'ap',
    label: 'Access Point',
    icon: Wifi,
    description:
      'Create a WiFi network for your devices to connect to. Best when using ethernet for internet.',
  },
  {
    mode: 'client',
    label: 'Client (STA)',
    icon: Monitor,
    description:
      'Connect to an existing WiFi network to get internet access. Your travel router acts as a WiFi client.',
  },
  {
    mode: 'repeater',
    label: 'Repeater',
    icon: Repeat,
    description:
      'Connect to an existing WiFi and rebroadcast it to your devices. Extends WiFi range.',
  },
];

export function WifiModeCard() {
  const { data: connection, isLoading } = useWifiConnection();
  const setMode = useWifiMode();
  const [pendingMode, setPendingMode] = useState<WifiMode | null>(null);

  const currentMode: WifiMode = connection?.mode ?? 'client';

  function handleConfirm() {
    if (pendingMode) {
      setMode.mutate(pendingMode);
      setPendingMode(null);
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">WiFi Mode</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="grid gap-3 sm:grid-cols-3">
              <Skeleton className="h-32 w-full" />
              <Skeleton className="h-32 w-full" />
              <Skeleton className="h-32 w-full" />
            </div>
          ) : (
            <div className="grid gap-3 sm:grid-cols-3">
              {MODE_OPTIONS.map(({ mode, label, icon: Icon, description }) => {
                const isActive = currentMode === mode;
                return (
                  <button
                    key={mode}
                    type="button"
                    disabled={setMode.isPending}
                    onClick={() => {
                      if (!isActive) setPendingMode(mode);
                    }}
                    className={cn(
                      'flex flex-col items-start gap-2 rounded-lg border p-4 text-left transition-colors',
                      isActive
                        ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-950'
                        : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:hover:border-gray-700 dark:hover:bg-gray-900',
                      setMode.isPending && 'opacity-50 cursor-not-allowed',
                    )}
                  >
                    <div className="flex w-full items-center justify-between">
                      <Icon
                        className={cn(
                          'h-5 w-5',
                          isActive
                            ? 'text-blue-600 dark:text-blue-400'
                            : 'text-gray-400',
                        )}
                      />
                      {isActive && <Badge variant="default">Active</Badge>}
                    </div>
                    <span
                      className={cn(
                        'text-sm font-medium',
                        isActive
                          ? 'text-blue-900 dark:text-blue-100'
                          : 'text-gray-900 dark:text-white',
                      )}
                    >
                      {label}
                    </span>
                    <p className="text-xs text-gray-500 dark:text-gray-400">{description}</p>
                  </button>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={pendingMode !== null} onOpenChange={(open) => !open && setPendingMode(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Switch WiFi Mode</DialogTitle>
            <DialogDescription>
              Are you sure you want to switch to{' '}
              <span className="font-medium text-gray-900 dark:text-white">
                {MODE_OPTIONS.find((o) => o.mode === pendingMode)?.label}
              </span>{' '}
              mode?
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
            <p className="text-sm text-amber-800 dark:text-amber-200">
              Changing the WiFi mode will restart the wireless subsystem. You may temporarily lose
              connectivity and need to reconnect to the router.
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingMode(null)}>
              Cancel
            </Button>
            <Button onClick={handleConfirm} disabled={setMode.isPending}>
              {setMode.isPending ? 'Switching...' : 'Switch Mode'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
