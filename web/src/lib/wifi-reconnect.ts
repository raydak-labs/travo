import type { ScanResult, WiFiProfileSummary } from './api/gen';

/**
 * The highest-priority saved uplink seen in the last scan. A BSSID-pinned profile
 * only counts when that access point was seen. 0.1.0 never switches automatically:
 * the operator presses Select (docs/adr/0014).
 */
export function suggestUplink(
  profiles: WiFiProfileSummary[],
  scan: ScanResult | undefined,
): WiFiProfileSummary | undefined {
  if (!scan) return undefined;
  return profiles.find((profile) =>
    scan.networks.some(
      (network) =>
        network.ssid === profile.ssid &&
        (!profile.bssid ||
          network.access_points.some(
            (ap) => ap.bssid.toLowerCase() === profile.bssid?.toLowerCase(),
          )),
    ),
  );
}
