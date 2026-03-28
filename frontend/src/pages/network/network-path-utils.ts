import type { NetworkSectionTab } from '@/pages/network/network-page-types';

/** Map URL to the Network page tab (routes: `/network`, `/network/configuration`, `/network/advanced`). */
export function networkPathnameToTab(pathname: string): NetworkSectionTab {
  const p = pathname.replace(/\/$/, '') || '/';
  if (p === '/network/configuration') return 'configuration';
  if (p === '/network/advanced') return 'advanced';
  return 'status';
}

export function networkTabToPath(tab: NetworkSectionTab): '/network' | '/network/configuration' | '/network/advanced' {
  if (tab === 'configuration') return '/network/configuration';
  if (tab === 'advanced') return '/network/advanced';
  return '/network';
}
