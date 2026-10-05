import { describe, expect, it } from 'vitest';
import { shellTitleForPath } from '@/components/layout/shell-titles';

describe('shellTitleForPath', () => {
  const cases: Array<[string, string]> = [
    ['/dashboard', 'Dashboard'],
    ['/wifi', 'Connect'],
    // Qualified: two pages both titled "Advanced" gave no way to tell the
    // section you were in from the header alone.
    ['/wifi/advanced', 'WiFi / Advanced'],
    ['/network', 'Status'],
    ['/network/configuration', 'Internet & LAN'],
    ['/network/advanced', 'Network / Advanced'],
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
