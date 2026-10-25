import { describe, expect, it } from 'vitest';
import type { UpdateStatus } from '@shared/index';
import { luciUpgradeUrl, summarizeUpdates, travoVersionUrl } from '../updates';

function status(over: Partial<UpdateStatus> = {}): UpdateStatus {
  return {
    travo: { current: '0.9.0' },
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
      travo: null,
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

  it('names a Travo release first', () => {
    const s = summarizeUpdates(
      status({
        travo: { current: '0.9.0', latest: '0.10.0', release_url: 'https://x/v0.10.0' },
        packages: { manager: 'apk', upgradable: [pkg] },
      }),
    );
    expect(s.travo).toBe('0.10.0');
    expect(s.label).toBe('Travo 0.10.0 and 1 package update');
  });

  it('joins three kinds with commas', () => {
    expect(
      summarizeUpdates(
        status({
          travo: { current: '0.9.0', latest: '0.10.0' },
          firmware: {
            current: '25.12.3',
            latest: '25.12.6',
            kind: 'patch',
            attended_sysupgrade: true,
          },
          packages: { manager: 'apk', upgradable: [pkg] },
        }),
      ).label,
    ).toBe('Travo 0.10.0, OpenWrt 25.12.6 and 1 package update');
  });
});

describe('travoVersionUrl', () => {
  const repo = 'https://github.com/raydak-labs/travo';
  it('links a release to its release page', () => {
    expect(travoVersionUrl('0.9.0')).toBe(`${repo}/releases/tag/v0.9.0`);
    // A build of the tag itself with local changes (seen on the router).
    expect(travoVersionUrl('v0.9.0-dirty')).toBe(`${repo}/releases/tag/v0.9.0`);
  });
  // A development build is `git describe` output; its commit is the useful link.
  it('links a development build to its commit', () => {
    expect(travoVersionUrl('v0.9.0-3-g2705c43-dirty')).toBe(`${repo}/commit/2705c43`);
    expect(travoVersionUrl('2705c43')).toBe(`${repo}/commit/2705c43`);
  });
  it('links an unknown version to the repository', () => {
    expect(travoVersionUrl('dev')).toBe(repo);
    expect(travoVersionUrl('')).toBe(repo);
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
