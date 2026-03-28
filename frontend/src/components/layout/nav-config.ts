import type { LucideIcon } from 'lucide-react';
import {
  Activity,
  Globe,
  Monitor,
  Network,
  ScrollText,
  Settings,
  Share2,
  Shield,
  Users,
  Wifi,
} from 'lucide-react';

/** Leaf nav entry (single route). */
export interface NavLeaf {
  readonly kind: 'leaf';
  readonly id: string;
  readonly to: string;
  readonly label: string;
  readonly icon: LucideIcon;
}

/** Group of related routes (collapsible in sidebar). */
export interface NavGroup {
  readonly kind: 'group';
  readonly id: string;
  readonly label: string;
  readonly icon: LucideIcon;
  readonly items: readonly { readonly to: string; readonly label: string }[];
}

export type NavEntry = NavLeaf | NavGroup;

/** Ordered sidebar structure: categories and sub-routes. */
export const NAV_ENTRIES: readonly NavEntry[] = [
  {
    kind: 'leaf',
    id: 'dashboard',
    to: '/dashboard',
    label: 'Dashboard',
    icon: Activity,
  },
  {
    kind: 'group',
    id: 'connectivity',
    label: 'Connectivity',
    icon: Network,
    items: [
      { to: '/wifi', label: 'WiFi' },
      { to: '/network', label: 'Network' },
      { to: '/clients', label: 'Clients' },
    ],
  },
  {
    kind: 'leaf',
    id: 'vpn',
    to: '/vpn',
    label: 'VPN',
    icon: Shield,
  },
  {
    kind: 'group',
    id: 'services',
    label: 'Services',
    icon: Monitor,
    items: [
      { to: '/services', label: 'Installed services' },
      { to: '/services/tailscale', label: 'Tailscale' },
    ],
  },
  {
    kind: 'group',
    id: 'system',
    label: 'System',
    icon: Settings,
    items: [
      { to: '/system', label: 'Settings' },
      { to: '/logs', label: 'Logs' },
    ],
  },
] as const;

/** Icon per route for collapsed rail and group rows. */
export const NAV_SUB_ICONS: Record<string, LucideIcon> = {
  '/wifi': Wifi,
  '/network': Globe,
  '/clients': Users,
  '/services': Monitor,
  '/services/tailscale': Share2,
  '/system': Settings,
  '/logs': ScrollText,
};

const STORAGE_KEY_GROUPS = 'otg-sidebar-groups';

/** Default open state for groups (first visit). */
const defaultOpen: Record<string, boolean> = {
  connectivity: true,
  services: true,
  system: true,
};

export function loadSidebarGroupState(): Record<string, boolean> {
  if (typeof window === 'undefined') return { ...defaultOpen };
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY_GROUPS);
    if (!raw) return { ...defaultOpen };
    const parsed = JSON.parse(raw) as Record<string, boolean>;
    return { ...defaultOpen, ...parsed };
  } catch {
    return { ...defaultOpen };
  }
}

export function saveSidebarGroupState(id: string, open: boolean): void {
  if (typeof window === 'undefined') return;
  try {
    const prev = loadSidebarGroupState();
    prev[id] = open;
    window.localStorage.setItem(STORAGE_KEY_GROUPS, JSON.stringify(prev));
  } catch {
    /* ignore quota / private mode */
  }
}

/** Whether `pathname` should highlight the nav link for `navTo`. */
export function isRouteActive(navTo: string, pathname: string): boolean {
  if (navTo === '/services') {
    return pathname === '/services';
  }
  return pathname === navTo || pathname.startsWith(`${navTo}/`);
}

/** Flat list of routes for collapsed icon rail (one icon per destination). */
export function flattenNavRoutes(): { to: string; label: string; icon: LucideIcon }[] {
  const out: { to: string; label: string; icon: LucideIcon }[] = [];
  for (const entry of NAV_ENTRIES) {
    if (entry.kind === 'leaf') {
      out.push({ to: entry.to, label: entry.label, icon: entry.icon });
    } else {
      for (const item of entry.items) {
        const icon = NAV_SUB_ICONS[item.to] ?? entry.icon;
        out.push({ to: item.to, label: item.label, icon });
      }
    }
  }
  return out;
}
