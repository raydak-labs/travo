import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useWifiConnection, useWifiMode } from '@/hooks/use-wifi';
import { cn } from '@/lib/cn';
import type { WifiMode } from '@shared/index';
import {
  isRecommendedWifiMode,
  WIFI_MODE_OPTIONS,
  getWifiModeLabel,
} from '@/components/wifi/wifi-mode-options';
import { WifiModeSwitchDialog } from '@/components/wifi/wifi-mode-switch-dialog';
import { WifiLockoutDialog } from '@/components/wifi/wifi-lockout-dialog';
import { useWifiLockout } from '@/hooks/use-wifi-lockout';
import { OperationProgressDialog } from '@/components/ui/operation-progress-dialog';

export function WifiModeCard() {
  const { data: connection, isLoading } = useWifiConnection();
  const setMode = useWifiMode();
  const lockout = useWifiLockout();
  const [pendingMode, setPendingMode] = useState<WifiMode | null>(null);
  const [switchingLabel, setSwitchingLabel] = useState<string | null>(null);

  const currentMode: WifiMode = connection?.mode ?? 'client';

  // One place sends the request, with or without the acknowledgement, so the
  // re-send after the dialog is the SAME request and not a rebuilt one that
  // could differ in a field the operator never changed.
  function submitMode(mode: WifiMode, acknowledge: boolean) {
    setSwitchingLabel(getWifiModeLabel(mode));
    setMode.mutate(
      { mode, acknowledge_lockout: acknowledge },
      {
        onSettled: () => {
          setPendingMode(null);
          setSwitchingLabel(null);
        },
        onError: (error) => {
          if (acknowledge) return;
          lockout.onLockout(error, () => submitMode(mode, true));
        },
      },
    );
    setPendingMode(null);
  }

  function handleConfirm() {
    if (!pendingMode) return;
    submitMode(pendingMode, false);
  }

  return (
    <>
      {/* finalizeWifiMutation blocks for up to 30s while it confirms the apply
          token. Without this the click produced three greyed-out tiles and no
          output at all, on the one page whose connectivity is about to drop. */}
      <OperationProgressDialog
        open={setMode.isPending}
        title={`Switching to ${switchingLabel ?? 'new'} mode…`}
        description="The router restarts its wireless subsystem and waits for the new settings to come up."
        details={[
          'This can take up to 30 seconds.',
          'Keep this page open — your browser confirms the change while the old configuration is still active.',
          'If the new mode does not come up, the router rolls back to the previous mode on its own.',
        ]}
      />
      <Card>
        <CardHeader>
          <CardTitle>WiFi Mode</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="mx-auto grid max-w-5xl gap-3 md:grid-cols-3">
              <Skeleton className="h-32 w-full" />
              <Skeleton className="h-32 w-full" />
              <Skeleton className="h-32 w-full" />
            </div>
          ) : (
            <div className="mx-auto grid max-w-5xl grid-cols-1 gap-3 md:grid-cols-3">
              {WIFI_MODE_OPTIONS.map(({ mode, label, icon: Icon, description }) => {
                const isActive = currentMode === mode;
                const showRecommended = isRecommendedWifiMode(mode);
                return (
                  <button
                    key={mode}
                    type="button"
                    disabled={setMode.isPending}
                    onClick={() => {
                      if (!isActive) {
                        setPendingMode(mode);
                      }
                    }}
                    className={cn(
                      'flex min-w-0 flex-col items-start gap-2 overflow-visible rounded-lg border p-4 text-left transition-colors',
                      isActive
                        ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-950'
                        : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50 dark:border-white/10 dark:hover:border-white/20 dark:hover:bg-gray-900',
                      setMode.isPending && 'opacity-50 cursor-not-allowed',
                    )}
                  >
                    <Icon
                      className={cn(
                        'h-5 w-5 shrink-0',
                        isActive ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400',
                      )}
                    />
                    <div className="flex w-full min-w-0 flex-wrap items-center gap-1.5">
                      <span
                        className={cn(
                          'min-w-0 text-sm font-medium',
                          isActive
                            ? 'text-blue-900 dark:text-blue-100'
                            : 'text-gray-900 dark:text-white',
                        )}
                      >
                        {label}
                      </span>
                      {showRecommended && (
                        <Badge variant="secondary" className="shrink-0 text-xs">
                          Recommended
                        </Badge>
                      )}
                      {isActive && (
                        <Badge variant="default" className="shrink-0">
                          Active
                        </Badge>
                      )}
                    </div>
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
        currentMode={currentMode}
        targetMode={pendingMode}
        isPending={setMode.isPending}
        onOpenChange={(open) => {
          if (!open) setPendingMode(null);
        }}
        onConfirm={handleConfirm}
      />

      <WifiLockoutDialog
        open={lockout.open}
        isPending={setMode.isPending}
        onCancel={lockout.dismiss}
        onConfirm={lockout.acknowledge}
      />
    </>
  );
}
