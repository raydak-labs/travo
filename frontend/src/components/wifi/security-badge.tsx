import { Badge } from '@/components/ui/badge';
import { Lock, Unlock } from 'lucide-react';

interface SecurityBadgeProps {
  /** Encryption from API (e.g. psk2, sae) or WifiEncryption */
  encryption: string;
}

const variantMap: Record<string, 'success' | 'warning' | 'destructive' | 'default'> = {
  wpa3: 'success',
  'wpa2/wpa3': 'success',
  wpa2: 'warning',
  psk2: 'warning',
  wpa: 'warning',
  psk: 'warning',
  wep: 'destructive',
  none: 'destructive',
  sae: 'success',
  'psk-mixed': 'success',
};

const labelMap: Record<string, string> = {
  wpa3: 'WPA3',
  'wpa2/wpa3': 'WPA2/3',
  wpa2: 'WPA2',
  psk2: 'WPA2',
  wpa: 'WPA',
  psk: 'WPA',
  wep: 'WEP',
  none: 'Open',
  sae: 'WPA3',
  'psk-mixed': 'WPA2/3',
};

function normalizeEncryption(enc: string): string {
  const lower = enc.toLowerCase();
  if (lower === 'sae') return 'wpa3';
  if (lower === 'psk2' || lower === 'psk-mixed') return lower === 'psk-mixed' ? 'wpa2/wpa3' : 'wpa2';
  return lower;
}

export function SecurityBadge({ encryption }: SecurityBadgeProps) {
  const norm = normalizeEncryption(encryption);
  const variant = variantMap[norm] ?? 'default';
  const label = labelMap[norm] ?? (encryption || 'Unknown');
  const Icon = norm === 'none' ? Unlock : Lock;

  return (
    <Badge variant={variant} data-testid="security-badge" className="gap-1">
      <Icon className="h-3 w-3 shrink-0" />
      {label}
    </Badge>
  );
}
