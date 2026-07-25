export function shellTitleForPath(pathname: string): string {
  const map: Record<string, string> = {
    '/dashboard': 'Dashboard',
    '/wifi': 'Connect',
    '/wifi/advanced': 'Advanced',
    '/network': 'Status',
    '/network/configuration': 'Internet & LAN',
    '/network/advanced': 'Advanced',
    '/clients': 'Clients',
    '/vpn': 'VPN',
    '/services': 'Services',
    '/services/tailscale': 'Services / Tailscale',
    '/services/speedtest': 'Services / Speedtest',
    '/services/sqm': 'Services / SQM',
    '/system': 'System',
    '/logs': 'Logs',
  };
  return map[pathname] ?? 'Travo';
}
