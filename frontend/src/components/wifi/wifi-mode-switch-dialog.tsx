import { useState } from 'react';
import { AlertTriangle, Info } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useConnectionMethod } from '@/hooks/use-network';
import { cn } from '@/lib/cn';
import type { WifiMode } from '@shared/index';
import { getWifiModeLabel } from '@/components/wifi/wifi-mode-options';

interface WifiModeSwitchDialogProps {
  open: boolean;
  currentMode: WifiMode;
  targetMode: WifiMode | null;
  isPending: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export function WifiModeSwitchDialog({
  open,
  currentMode,
  targetMode,
  isPending,
  onOpenChange,
  onConfirm,
}: WifiModeSwitchDialogProps) {
  const { data: connectionMethod } = useConnectionMethod();
  const [showDetails, setShowDetails] = useState(false);

  const isWifiClient = connectionMethod?.method === 'wifi-client';
  const isWifiAp = connectionMethod?.method === 'wifi-ap';

  function getWarningInfo() {
    if (!targetMode) return null;

    if (currentMode === 'repeater' && targetMode === 'client') {
      return {
        severity: isWifiAp ? 'high' : 'medium',
        title: isWifiAp ? 'Risk of lockout' : 'Local Wi-Fi will turn off',
        message: isWifiAp
          ? "Client mode disables your router's Wi-Fi access point. You may lose access to this page."
          : 'Client mode disables the Wi-Fi network you broadcast. Devices can still use Ethernet on LAN.',
        details: [
          'The wireless subsystem will restart.',
          'If you need local Wi-Fi for phones and laptops, use Travel / repeater or Access Point mode instead.',
          'Changes will be confirmed within 30 seconds or rolled back.',
        ],
        extraWarning: isWifiAp
          ? 'Connect via Ethernet before proceeding, or you may be unable to reach the router.'
          : null,
      };
    }

    if (currentMode === 'repeater' && targetMode === 'ap') {
      return {
        severity: 'high' as const,
        title: 'Will lose Wi-Fi internet',
        message:
          'Access Point mode does not join upstream Wi-Fi. Your router will only offer its own network.',
        details: [
          'Use Ethernet (or another WAN source) for internet.',
          'If you were using hotel or public Wi-Fi through this router, that path will stop.',
          'The AP SSID for your devices stays available if it was already configured.',
        ],
        extraWarning: isWifiClient
          ? 'You appear to reach this device via Wi-Fi WAN; confirm you have Ethernet or AP access before switching.'
          : null,
      };
    }

    if (currentMode === 'ap' && targetMode === 'repeater') {
      return {
        severity: 'medium' as const,
        title: 'Connection may drop temporarily',
        message: 'Travel / repeater mode enables Wi-Fi client (STA) as well as your access point.',
        details: [
          'The wireless subsystem will restart.',
          'You can connect to upstream Wi-Fi for internet while keeping your own Wi-Fi for devices.',
          'This usually takes 30–60 seconds.',
        ],
        extraWarning: null,
      };
    }

    if (currentMode === 'ap' && targetMode === 'client') {
      return {
        severity: isWifiAp ? 'high' : 'medium',
        title: isWifiAp ? 'Risk of lockout' : 'Wi-Fi access point will turn off',
        message: isWifiAp
          ? 'Client mode has no local Wi-Fi access point. You may lose access to this page.'
          : 'Client mode disables the Wi-Fi network clients join. Use Ethernet on LAN to manage the router.',
        details: [
          'The router will only join upstream Wi-Fi for internet (no broadcast SSID for your LAN).',
          'The wireless subsystem will restart.',
          'Changes will be confirmed within 30 seconds or rolled back.',
        ],
        extraWarning: isWifiAp
          ? 'Connect via Ethernet before proceeding, or you may be unable to reach the router.'
          : null,
      };
    }

    if (currentMode === 'client' && targetMode === 'ap') {
      return {
        severity: 'high' as const,
        title: 'Will lose internet access',
        message: 'Switching to AP mode will disconnect your WiFi client connection.',
        details: [
          'Your device will no longer connect to a WiFi network for internet.',
          'Connect via Ethernet to the LAN port, or connect to the AP WiFi network.',
          'The AP SSID is typically "Travo-Setup" or similar.',
        ],
        extraWarning: isWifiClient
          ? 'You are currently connected via WiFi and will be locked out without Ethernet or AP access.'
          : null,
      };
    }

    if (currentMode === 'client' && targetMode === 'repeater') {
      return {
        severity: 'medium' as const,
        title: 'Connection may drop temporarily',
        message: 'Switching to Travel / repeater mode may cause your connection to drop.',
        details: [
          'The wireless subsystem will restart.',
          'You may need to reconnect to the router via the new AP network.',
          'This usually takes 30-60 seconds.',
        ],
        extraWarning: null,
      };
    }

    if (targetMode === 'ap' && isWifiClient) {
      return {
        severity: 'high' as const,
        title: 'Risk of lockout',
        message: 'You are connected via WiFi client. You may lose access.',
        details: [
          'AP mode disables WiFi client functionality.',
          'Ensure you have Ethernet access or can connect to the AP WiFi.',
          'Enable Emergency AP in advanced settings for guaranteed access.',
        ],
        extraWarning:
          'Strongly recommended: Enable Emergency AP or connect via Ethernet before proceeding.',
      };
    }

    return {
      severity: 'low' as const,
      title: 'Wireless will restart',
      message: 'Changing WiFi mode requires restarting the wireless subsystem.',
      details: [
        'Your connection may drop temporarily.',
        'Reconnect via WiFi or Ethernet if needed.',
        'Changes will be confirmed within 30 seconds or rolled back.',
      ],
      extraWarning: null,
    };
  }

  if (!targetMode) return null;

  const warning = getWarningInfo();
  const modeLabel = getWifiModeLabel(targetMode);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Switch to {modeLabel} mode</DialogTitle>
          <DialogDescription>
            Are you sure you want to switch WiFi mode to{' '}
            <span className="font-medium text-gray-900 dark:text-white">{modeLabel}</span>?
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          <div
            className={cn(
              'flex items-start gap-3 rounded-lg border p-3',
              warning!.severity === 'high'
                ? 'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950'
                : warning!.severity === 'medium'
                  ? 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950'
                  : 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950',
            )}
          >
            <AlertTriangle
              className={cn(
                'mt-0.5 h-4 w-4 shrink-0',
                warning!.severity === 'high'
                  ? 'text-red-600 dark:text-red-400'
                  : warning!.severity === 'medium'
                    ? 'text-amber-600 dark:text-amber-400'
                    : 'text-blue-600 dark:text-blue-400',
              )}
            />
            <div className="flex-1">
              <p
                className={cn(
                  'text-sm font-medium',
                  warning!.severity === 'high'
                    ? 'text-red-900 dark:text-red-100'
                    : warning!.severity === 'medium'
                      ? 'text-amber-900 dark:text-amber-100'
                      : 'text-blue-900 dark:text-blue-100',
                )}
              >
                {warning!.title}
              </p>
              <p
                className={cn(
                  'text-xs mt-1',
                  warning!.severity === 'high'
                    ? 'text-red-800 dark:text-red-200'
                    : warning!.severity === 'medium'
                      ? 'text-amber-800 dark:text-amber-200'
                      : 'text-blue-800 dark:text-blue-200',
                )}
              >
                {warning!.message}
              </p>
            </div>
          </div>

          {warning!.extraWarning && (
            <div className="rounded-lg border border-red-300 bg-red-100 p-3 dark:border-red-700 dark:bg-red-900">
              <p className="text-sm font-medium text-red-900 dark:text-red-100">
                {warning!.extraWarning}
              </p>
            </div>
          )}

          <button
            type="button"
            onClick={() => setShowDetails(!showDetails)}
            className="flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white"
          >
            <Info className="h-4 w-4" />
            {showDetails ? 'Hide details' : 'Show more details'}
          </button>

          {showDetails && (
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-900">
              <ul className="space-y-2 text-sm text-gray-700 dark:text-gray-300">
                {warning!.details.map((detail, index) => (
                  <li key={index} className="flex items-start gap-2">
                    <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-gray-400 dark:bg-gray-600" />
                    {detail}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950">
            <p className="text-sm text-blue-800 dark:text-blue-200">
              <span className="font-medium">Keep this page open:</span> Your browser will confirm
              the router is still reachable. If successful, the mode updates automatically. If the
              rollback triggers, you will see a connection error — refresh the page and your old
              mode will be restored.
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} type="button">
            Cancel
          </Button>
          <Button
            onClick={onConfirm}
            disabled={isPending}
            type="button"
            className={warning!.severity === 'high' ? 'bg-red-600 hover:bg-red-700' : undefined}
          >
            {isPending ? 'Switching...' : 'I understand, switch mode'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
