import type { WifiHiddenNetworkFormValues } from '@/lib/schemas/wifi-forms';

export const WIFI_HIDDEN_ENCRYPTION_OPTIONS = [
  { value: 'psk2', label: 'WPA2 (PSK)' },
  { value: 'sae', label: 'WPA3 (SAE)' },
  { value: 'psk', label: 'WPA (PSK)' },
  { value: 'none', label: 'Open (No Password)' },
] as const;

export const wifiHiddenNetworkDefaultValues: WifiHiddenNetworkFormValues = {
  ssid: '',
  encryption: 'psk2',
  password: '',
};
