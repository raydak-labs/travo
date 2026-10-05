import type { LucideIcon } from 'lucide-react';
import { Shield, ShieldCheck, ShieldBan, Globe, ArrowLeftRight, Eye, Cloud } from 'lucide-react';
import type { ServiceState } from '@shared/index';
import type { StatusPillProps } from '@/components/ui/status-pill';

export const serviceCardIcons: Record<string, LucideIcon> = {
  wireguard: Shield,
  tailscale: ShieldCheck,
  adguardhome: ShieldBan,
  openvpn: Globe,
  mwan3: ArrowLeftRight,
  watchcat: Eye,
  cloudflared: Cloud,
};

/** A stopped service is a choice, not yet a problem; only `error` reads red. */
export const serviceStateTone: Record<ServiceState, StatusPillProps['tone']> = {
  running: 'ok',
  installed: 'info',
  stopped: 'warn',
  not_installed: 'neutral',
  error: 'danger',
};

export const serviceStateLabels: Record<ServiceState, string> = {
  running: 'Running',
  installed: 'Installed',
  stopped: 'Stopped',
  not_installed: 'Not Installed',
  error: 'Error',
};
