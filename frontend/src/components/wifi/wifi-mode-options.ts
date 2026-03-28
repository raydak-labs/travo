import type { LucideIcon } from 'lucide-react';
import { Monitor, Repeat, Wifi } from 'lucide-react';
import type { WifiMode } from '@shared/index';

export interface WifiModeOption {
  mode: WifiMode;
  label: string;
  icon: LucideIcon;
  description: string;
}

export const WIFI_MODE_OPTIONS: WifiModeOption[] = [
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

export function getWifiModeLabel(mode: WifiMode): string {
  return WIFI_MODE_OPTIONS.find((o) => o.mode === mode)?.label ?? mode;
}
