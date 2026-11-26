import type { Component } from 'svelte';
import type { Route } from '../router';

export type PageComponent = Component<Record<string, unknown>>;
export type PageImport = () => Promise<PageComponent>;

/**
 * Wraps a dynamic import so each page chunk is requested once. A rejected import is not cached,
 * so a retry after a network failure fetches the chunk again.
 */
export function lazyPage(importer: () => Promise<{ default: unknown }>): PageImport {
  let cached: Promise<PageComponent> | undefined;
  return () => {
    cached ??= importer().then(
      (module) => module.default as PageComponent,
      (error) => {
        cached = undefined;
        throw error;
      },
    );
    return cached;
  };
}

/** One chunk per route; the shell picks the page by the guarded route. */
export const pages: Record<Route, PageImport> = {
  dashboard: lazyPage(() => import('./Dashboard.svelte')),
  login: lazyPage(() => import('./Login.svelte')),
  setup: lazyPage(() => import('./Setup.svelte')),
  internet: lazyPage(() => import('./Internet.svelte')),
  wifi: lazyPage(() => import('./WiFi.svelte')),
  vpn: lazyPage(() => import('./VPN.svelte')),
  'vpn-wireguard': lazyPage(() => import('./WireGuard.svelte')),
  'vpn-tailscale': lazyPage(() => import('./Tailscale.svelte')),
  clients: lazyPage(() => import('./Clients.svelte')),
  services: lazyPage(() => import('./Services.svelte')),
  'services-adguard': lazyPage(() => import('./AdGuard.svelte')),
  'network-lan': lazyPage(() => import('./NetworkLAN.svelte')),
  'network-port-forwards': lazyPage(() => import('./NetworkPortForwards.svelte')),
  'network-ipv6': lazyPage(() => import('./NetworkIPv6.svelte')),
  'network-diagnostics': lazyPage(() => import('./NetworkDiagnostics.svelte')),
  system: lazyPage(() => import('./System.svelte')),
  'system-hardware': lazyPage(() => import('./SystemHardware.svelte')),
  'system-logs': lazyPage(() => import('./SystemLogs.svelte')),
  'system-firmware': lazyPage(() => import('./SystemFirmware.svelte')),
  'system-access': lazyPage(() => import('./SystemAccess.svelte')),
};
