import type { UpdateStatus } from '@shared/index';

export const TRAVO_REPO_URL = 'https://github.com/raydak-labs/travo';

export interface UpdateSummary {
  readonly any: boolean;
  /** The Travo release on offer, or null. */
  readonly travo: string | null;
  /** The OpenWrt release on offer, or null when the device runs the newest. */
  readonly firmware: string | null;
  readonly packages: number;
  /** "Travo 0.10.0, OpenWrt 25.12.6 and 2 package updates"; empty when nothing is on offer. */
  readonly label: string;
}

/** What the last update check found, in the words the header and menu use. */
export function summarizeUpdates(status: UpdateStatus | undefined): UpdateSummary {
  const travo = status?.travo.latest || null;
  const firmware = status?.firmware.latest || null;
  const packages = status?.packages.upgradable.length ?? 0;
  const parts = [
    travo ? `Travo ${travo}` : '',
    firmware ? `OpenWrt ${firmware}` : '',
    packages ? `${packages} package update${packages === 1 ? '' : 's'}` : '',
  ].filter(Boolean);
  const label =
    parts.length > 1 ? `${parts.slice(0, -1).join(', ')} and ${parts.at(-1)}` : (parts[0] ?? '');
  return { any: parts.length > 0, travo, firmware, packages, label };
}

/**
 * Where to flash a new OpenWrt release. Travo does not flash releases itself:
 * LuCI's Attended Sysupgrade builds an image that keeps the installed packages.
 * LuCI listens on 8080 once the installer moved it (99-travel-gui-ports).
 */
export function luciUpgradeUrl(hostname: string, attendedSysupgrade: boolean): string {
  const page = attendedSysupgrade ? 'attendedsysupgrade' : 'flash';
  return `http://${hostname}:8080/cgi-bin/luci/admin/system/${page}`;
}

/**
 * Where a Travo version lives on GitHub: a release's page, a development
 * build's commit (its version is `git describe` output such as
 * v0.9.0-3-g2705c43-dirty), else the repository.
 */
export function travoVersionUrl(version: string): string {
  const tag = /^v?(\d+\.\d+\.\d+)(?:-dirty)?$/.exec(version);
  if (tag) return `${TRAVO_REPO_URL}/releases/tag/v${tag[1]}`;
  const commit =
    /-g([0-9a-f]{7,40})(?:-dirty)?$/.exec(version) ?? /^([0-9a-f]{7,40})(?:-dirty)?$/.exec(version);
  return commit ? `${TRAVO_REPO_URL}/commit/${commit[1]}` : TRAVO_REPO_URL;
}

/** "v0.9.0" for a release; a development build's description as it is. */
export function formatTravoVersion(version: string): string {
  return /^\d+\.\d+\.\d+$/.test(version) ? `v${version}` : version;
}
