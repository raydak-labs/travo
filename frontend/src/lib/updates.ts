import type { UpdateStatus } from '@shared/index';

export interface UpdateSummary {
  readonly any: boolean;
  /** The OpenWrt release on offer, or null when the device runs the newest. */
  readonly firmware: string | null;
  readonly packages: number;
  /** "OpenWrt 25.12.6 and 2 package updates"; empty when nothing is on offer. */
  readonly label: string;
}

/** What the last update check found, in the words the header and menu use. */
export function summarizeUpdates(status: UpdateStatus | undefined): UpdateSummary {
  const firmware = status?.firmware.latest || null;
  const packages = status?.packages.upgradable.length ?? 0;
  const parts = [
    firmware ? `OpenWrt ${firmware}` : '',
    packages ? `${packages} package update${packages === 1 ? '' : 's'}` : '',
  ].filter(Boolean);
  return { any: parts.length > 0, firmware, packages, label: parts.join(' and ') };
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
