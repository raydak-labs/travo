import { describe, expect, it } from 'vitest';
import type { UpdateStatus } from '@shared/index';
import { luciUpgradeUrl, summarizeUpdates } from '../updates';

function status(over: Partial<UpdateStatus> = {}): UpdateStatus {
  return {
    firmware: { current: '25.12.3', attended_sysupgrade: true },
    packages: { manager: 'apk', upgradable: [] },
    checked_at: '2026-10-25T15:00:00Z',
    errors: [],
    ...over,
  };
}

const pkg = { name: 'htop', current: '3.4.1-r1', available: '3.5.1-r1' };

describe('summarizeUpdates', () => {
  it('reports nothing before the first check or when up to date', () => {
    expect(summarizeUpdates(undefined).any).toBe(false);
    expect(summarizeUpdates(status()).any).toBe(false);
  });

  it('names the firmware release and counts the packages', () => {
    const s = summarizeUpdates(
      status({
        firmware: {
          current: '25.12.3',
          latest: '25.12.6',
          kind: 'patch',
          attended_sysupgrade: true,
        },
        packages: { manager: 'apk', upgradable: [pkg, { ...pkg, name: 'curl' }] },
      }),
    );
    expect(s).toEqual({
      any: true,
      firmware: '25.12.6',
      packages: 2,
      label: 'OpenWrt 25.12.6 and 2 package updates',
    });
  });

  it('words a single update of each kind', () => {
    expect(
      summarizeUpdates(status({ packages: { manager: 'apk', upgradable: [pkg] } })).label,
    ).toBe('1 package update');
    expect(
      summarizeUpdates(
        status({
          firmware: {
            current: '24.10.8',
            latest: '25.12.6',
            kind: 'release',
            attended_sysupgrade: false,
          },
        }),
      ).label,
    ).toBe('OpenWrt 25.12.6');
  });
});

describe('luciUpgradeUrl', () => {
  // LuCI was moved to 8080 by the installer (99-travel-gui-ports).
  it('opens Attended Sysupgrade when it is installed, else the flash page', () => {
    expect(luciUpgradeUrl('192.168.1.1', true)).toBe(
      'http://192.168.1.1:8080/cgi-bin/luci/admin/system/attendedsysupgrade',
    );
    expect(luciUpgradeUrl('192.168.1.1', false)).toBe(
      'http://192.168.1.1:8080/cgi-bin/luci/admin/system/flash',
    );
  });
});
