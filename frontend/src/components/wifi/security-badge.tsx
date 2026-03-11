import type { WifiEncryption } from '@shared/index';
import { Badge } from '@/components/ui/badge';
import { Lock, Unlock } from 'lucide-react';

interface SecurityBadgeProps {
  encryption: WifiEncryption;
}

const variantMap: Record<WifiEncryption, 'success' | 'warning' | 'destructive' | 'default'> = {
  wpa3: 'success',
  'wpa2/wpa3': 'success',
  wpa2: 'warning',
  wpa: 'warning',
  wep: 'destructive',
  none: 'destructive',
};

const labelMap: Record<WifiEncryption, string> = {
  wpa3: 'WPA3',
  'wpa2/wpa3': 'WPA2/3',
  wpa2: 'WPA2',
  wpa: 'WPA',
  wep: 'WEP',
  none: 'Open',
};

export function SecurityBadge({ encryption }: SecurityBadgeProps) {
  const variant = variantMap[encryption];
  const label = labelMap[encryption];
  const Icon = encryption === 'none' ? Unlock : Lock;

  return (
    <Badge variant={variant} data-testid="security-badge" className="gap-1">
      <Icon className="h-3 w-3 shrink-0" />
      {label}
    </Badge>
  );
}
