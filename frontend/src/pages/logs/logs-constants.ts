import type { LogsFilterFormValues } from '@/lib/schemas/logs-forms';

export const LOG_SERVICE_FILTERS = [
  { label: 'All', value: '' },
  { label: 'dnsmasq', value: 'dnsmasq' },
  { label: 'AdGuardHome', value: 'AdGuardHome' },
  { label: 'wireguard', value: 'wireguard' },
  { label: 'netifd', value: 'netifd' },
  { label: 'hostapd', value: 'hostapd' },
  { label: 'dropbear', value: 'dropbear' },
] as const;

export const LOG_LEVEL_FILTERS: { label: string; value: string }[] = [
  { label: 'All Levels', value: '' },
  { label: 'Error & above', value: 'err' },
  { label: 'Warning & above', value: 'warning' },
  { label: 'Notice & above', value: 'notice' },
  { label: 'Info & above', value: 'info' },
  { label: 'Debug (all)', value: 'debug' },
];

export const LOG_LEVEL_COLORS: Record<string, string> = {
  emerg: 'bg-red-600 text-white',
  alert: 'bg-red-500 text-white',
  crit: 'bg-red-500 text-white',
  err: 'bg-red-400 text-white',
  warning: 'bg-amber-400 text-amber-950',
  notice: 'bg-green-400 text-green-950',
  info: 'bg-blue-400 text-white',
  debug: 'bg-gray-400 text-gray-950',
};

export const defaultLogsFilters: LogsFilterFormValues = {
  lineFilter: '',
  serviceFilter: '',
  levelFilter: '',
  customService: '',
  showCustomInput: false,
};

export type LogTab = 'system' | 'kernel';
