/**
 * Shared Wi‑Fi band display and grouping helpers. The API spells bands three
 * ways: UCI's `2g`/`5g`/`6g` (radios, APs, the uplink), scan's `5GHz`, and the
 * request's `5ghz`; every helper here accepts all of them.
 */

/** Human-readable band label for any band spelling (5g, 5ghz, 5GHz). */
export function formatWifiBandLabel(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g' || b === '2g') return '2.4 GHz';
  if (b === '5ghz' || b === '5g') return '5 GHz';
  if (b === '6ghz' || b === '6g') return '6 GHz';
  return band;
}

/** Canonical key for grouping / comparing bands (2.4ghz, 5ghz, 6ghz, or raw string). */
export function normalizeWifiBandKey(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g' || b === '2g') return '2.4ghz';
  if (b === '5ghz' || b === '5g') return '5ghz';
  if (b === '6ghz' || b === '6g') return '6ghz';
  return band;
}
