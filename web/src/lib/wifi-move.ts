import type { WiFiSummary } from './api/gen';

/**
 * SSIDs of the access points a join would move to the other radio, mirroring the
 * daemon's reconcile (invariants §10): only when every enabled, band-compatible radio
 * carries an AP. The first such radio by name is the one the STA takes. The daemon
 * stays authoritative; this only warns before the user presses Join.
 */
export function apsMovedByJoin(summary: WiFiSummary | undefined, bands: string[]): string[] {
  if (!summary) return [];
  const radios = summary.radios
    .filter((radio) => !radio.disabled && bands.includes(radio.band))
    .map((radio) => radio.name)
    .sort();
  const carrying = (name: string) => summary.aps.filter((ap) => ap.radio === name);
  const [first] = radios;
  if (first === undefined || radios.some((name) => carrying(name).length === 0)) return [];
  return carrying(first).map((ap) => ap.ssid);
}
