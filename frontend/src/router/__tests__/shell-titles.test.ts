import { describe, expect, it } from 'vitest';
import { shellTitleForPath } from '@/components/layout/shell-titles';

describe('shellTitleForPath', () => {
  const cases: Array<[string, string]> = [
    ['/dashboard', 'Dashboard'],
    ['/wifi', 'Connect'],
    ['/wifi/advanced', 'Advanced'],
    ['/network', 'Status'],
    ['/network/configuration', 'Internet & LAN'],
    ['/network/advanced', 'Advanced'],
    ['/clients', 'Clients'],
    ['/vpn', 'VPN'],
    ['/services', 'Services'],
    ['/services/tailscale', 'Services / Tailscale'],
    ['/services/speedtest', 'Services / Speedtest'],
    ['/services/sqm', 'Services / SQM'],
    ['/system', 'System'],
    ['/logs', 'Logs'],
  ];

  it.each(cases)('%s → %s', (pathname, title) => {
    expect(shellTitleForPath(pathname)).toBe(title);
  });

  it('falls back to Travo for unknown paths', () => {
    expect(shellTitleForPath('/unknown')).toBe('Travo');
  });
});
