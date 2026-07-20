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
  emerg: 'bg-red-600 text-white dark:bg-red-700 dark:text-red-100',
  alert: 'bg-red-500 text-white dark:bg-red-600 dark:text-red-100',
  crit: 'bg-red-500 text-white dark:bg-red-600 dark:text-red-100',
  err: 'bg-red-400 text-white dark:bg-red-500 dark:text-red-50',
  warning: 'bg-amber-400 text-amber-950 dark:bg-amber-500 dark:text-amber-950',
  notice: 'bg-emerald-400 text-emerald-950 dark:bg-emerald-500 dark:text-emerald-950',
  info: 'bg-blue-400 text-white dark:bg-blue-500 dark:text-blue-50',
  debug: 'bg-gray-400 text-gray-950 dark:bg-gray-500 dark:text-gray-950',
};

export const defaultLogsFilters: LogsFilterFormValues = {
  lineFilter: '',
  serviceFilter: '',
  levelFilter: '',
  customService: '',
  showCustomInput: false,
};

export type LogTab = 'system' | 'kernel';
