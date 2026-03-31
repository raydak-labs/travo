import type { LucideIcon } from 'lucide-react';
import {
  Activity,
  Globe,
  Monitor,
  ScrollText,
  Settings,
  Settings2,
  Share2,
  Shield,
  SlidersHorizontal,
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
    to: '/dashboard-1',
    label: 'Dashboard',
    icon: Activity,
  },
  {
    kind: 'leaf',
    id: 'dashboard-new',
    to: '/dashboard-2',
    label: 'Dashboard (NEW)',
    icon: Activity,
  },
  {
    kind: 'group',
    id: 'wifi',
    label: 'WiFi',
    icon: Wifi,
    items: [
      { to: '/wifi', label: 'Wireless' },
      { to: '/wifi/advanced', label: 'Advanced' },
    ],
  },
  {
    kind: 'group',
    id: 'network',
    label: 'Network',
    icon: Globe,
    items: [
      { to: '/network', label: 'Status' },
      { to: '/network/configuration', label: 'Configuration' },
      { to: '/network/advanced', label: 'Advanced' },
    ],
  },
  {
    kind: 'leaf',
    id: 'clients',
    to: '/clients',
    label: 'Clients',
    icon: Users,
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

const GROUP_CHILD_PATHS: ReadonlySet<string> = new Set([
  ...NAV_ENTRIES.flatMap((e) => (e.kind === 'group' ? e.items.map((i) => i.to) : [])),
  // Shown in the Services submenu only when SQM is installed; still a valid deep link.
  '/services/sqm',
]);

/** Icon per route for collapsed rail and group rows. */
export const NAV_SUB_ICONS: Record<string, LucideIcon> = {
  '/wifi': Wifi,
  '/wifi/advanced': SlidersHorizontal,
  '/network': Activity,
  '/network/configuration': Settings2,
  '/network/advanced': Shield,
  '/clients': Users,
  '/services': Monitor,
  '/services/tailscale': Share2,
  '/services/sqm': SlidersHorizontal,
  '/system': Settings,
  '/logs': ScrollText,
};

const STORAGE_KEY_GROUPS = 'otg-sidebar-groups';

/** Default open state for groups (first visit). */
const defaultOpen: Record<string, boolean> = {
  wifi: true,
  network: true,
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
  let path = pathname;
  if (path.length > 1 && path.endsWith('/')) path = path.slice(0, -1);

  if (navTo === '/services') {
    return path === '/services';
  }
  if (GROUP_CHILD_PATHS.has(navTo)) {
    return path === navTo;
  }
  return path === navTo || path.startsWith(`${navTo}/`);
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

/** Collapsed sidebar rail: include SQM under Services only when the package is installed. */
export function flattenNavRoutesWithSqm(sqmInstalled: boolean): {
  to: string;
  label: string;
  icon: LucideIcon;
}[] {
  const base = flattenNavRoutes();
  if (!sqmInstalled) return base;
  const tailscaleIdx = base.findIndex((r) => r.to === '/services/tailscale');
  const sqmEntry = {
    to: '/services/sqm',
    label: 'SQM',
    icon: NAV_SUB_ICONS['/services/sqm'] ?? Monitor,
  };
  if (tailscaleIdx === -1) {
    return [...base, sqmEntry];
  }
  return [...base.slice(0, tailscaleIdx + 1), sqmEntry, ...base.slice(tailscaleIdx + 1)];
}
