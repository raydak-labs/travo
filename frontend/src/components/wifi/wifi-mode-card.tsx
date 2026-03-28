import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useWifiConnection, useWifiMode } from '@/hooks/use-wifi';
import { cn } from '@/lib/cn';
import type { WifiMode } from '@shared/index';
import { RepeaterWizard } from '@/components/wifi/repeater-wizard';
import { getWifiModeLabel, WIFI_MODE_OPTIONS } from '@/components/wifi/wifi-mode-options';
import { WifiModeSwitchDialog } from '@/components/wifi/wifi-mode-switch-dialog';

export function WifiModeCard() {
  const { data: connection, isLoading } = useWifiConnection();
  const setMode = useWifiMode();
  const [pendingMode, setPendingMode] = useState<WifiMode | null>(null);
  const [wizardOpen, setWizardOpen] = useState(false);

  const currentMode: WifiMode = connection?.mode ?? 'client';

  function handleConfirm() {
    if (pendingMode) {
      setMode.mutate(pendingMode);
      setPendingMode(null);
    }
  }

  return (
    <>
      <RepeaterWizard open={wizardOpen} onOpenChange={setWizardOpen} />
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
              {WIFI_MODE_OPTIONS.map(({ mode, label, icon: Icon, description }) => {
                const isActive = currentMode === mode;
                return (
                  <button
                    key={mode}
                    type="button"
                    disabled={setMode.isPending}
                    onClick={() => {
                      if (!isActive) {
                        if (mode === 'repeater') {
                          setWizardOpen(true);
                        } else {
                          setPendingMode(mode);
                        }
                      }
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
                          isActive ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400',
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

      <WifiModeSwitchDialog
        open={pendingMode !== null}
        modeLabel={pendingMode != null ? getWifiModeLabel(pendingMode) : undefined}
        isPending={setMode.isPending}
        onOpenChange={(open) => {
          if (!open) setPendingMode(null);
        }}
        onConfirm={handleConfirm}
      />
    </>
  );
}
