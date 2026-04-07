import type { LucideIcon } from 'lucide-react';
import { Shield, ShieldCheck, ShieldBan, Globe, ArrowLeftRight } from 'lucide-react';
import type { ServiceState } from '@shared/index';

export const serviceCardIcons: Record<string, LucideIcon> = {
  wireguard: Shield,
  tailscale: ShieldCheck,
  adguardhome: ShieldBan,
  openvpn: Globe,
  mwan3: ArrowLeftRight,
};

export const serviceStateBadgeVariant: Record<
  ServiceState,
  'success' | 'warning' | 'outline' | 'destructive'
> = {
  running: 'success',
  installed: 'warning',
  stopped: 'warning',
  not_installed: 'outline',
  error: 'destructive',
};

export const serviceStateLabels: Record<ServiceState, string> = {
  running: 'Running',
  installed: 'Installed',
  stopped: 'Stopped',
  not_installed: 'Not Installed',
  error: 'Error',
};
