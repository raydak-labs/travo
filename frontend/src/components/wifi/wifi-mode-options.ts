import type { LucideIcon } from 'lucide-react';
import { Monitor, Radio, Wifi } from 'lucide-react';
import type { WifiMode } from '@shared/index';

export interface WifiModeOption {
  mode: WifiMode;
  label: string;
  icon: LucideIcon;
  description: string;
  /** Shown on the mode tile when not the active mode (e.g. default pick for travel routers). */
  recommended?: boolean;
}

export const WIFI_MODE_OPTIONS: WifiModeOption[] = [
  {
    mode: 'repeater',
    label: 'Travel / repeater',
    icon: Radio,
    recommended: true,
    description:
      'Broadcast your own Wi-Fi for devices and connect this router to hotel or public Wi-Fi for internet. Best default for a travel router.',
  },
  {
    mode: 'client',
    label: 'Client (STA)',
    icon: Monitor,
    description:
      'Use Wi-Fi only as internet (WAN). Your own Wi-Fi access point is off—connect computers via Ethernet to LAN, or use another mode if you need local Wi-Fi.',
  },
  {
    mode: 'ap',
    label: 'Access Point',
    icon: Wifi,
    description:
      'Only creates a Wi-Fi network for devices to join. Does not connect to other Wi-Fi networks—use Ethernet (or another WAN) for internet.',
  },
];

export function getWifiModeLabel(mode: WifiMode): string {
  return WIFI_MODE_OPTIONS.find((o) => o.mode === mode)?.label ?? mode;
}

export function isRecommendedWifiMode(mode: WifiMode): boolean {
  return WIFI_MODE_OPTIONS.find((o) => o.mode === mode)?.recommended ?? false;
}
