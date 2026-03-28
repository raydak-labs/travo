/**
 * Shared Wi‑Fi band display and grouping helpers (scan results, connect UI, etc.).
 * UCI radio roles like `2g` / `5g` use separate labels in AP-specific components.
 */

/** Human-readable band label for scan / API strings (e.g. 5ghz, 5GHz). */
export function formatWifiBandLabel(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4 GHz';
  if (b === '5ghz' || b === '5g') return '5 GHz';
  if (b === '6ghz' || b === '6g') return '6 GHz';
  return band;
}

/** Canonical key for grouping / comparing bands (2.4ghz, 5ghz, 6ghz, or raw string). */
export function normalizeWifiBandKey(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4ghz';
  if (b === '5ghz' || b === '5g') return '5ghz';
  if (b === '6ghz' || b === '6g') return '6ghz';
  return band;
}
