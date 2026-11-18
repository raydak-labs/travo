import type { Component } from 'svelte';

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

export const pages = {
  dashboard: lazyPage(() => import('./Dashboard.svelte')),
  wifi: lazyPage(() => import('./WiFi.svelte')),
  network: lazyPage(() => import('./Network.svelte')),
  vpn: lazyPage(() => import('./VPN.svelte')),
  services: lazyPage(() => import('./Services.svelte')),
  system: lazyPage(() => import('./System.svelte')),
  systemTokens: lazyPage(() => import('./SystemTokens.svelte')),
  vpnTailscale: lazyPage(() => import('./Tailscale.svelte')),
  login: lazyPage(() => import('./Login.svelte')),
  setup: lazyPage(() => import('./Setup.svelte')),
  notYet: lazyPage(() => import('./NotYet.svelte')),
};
