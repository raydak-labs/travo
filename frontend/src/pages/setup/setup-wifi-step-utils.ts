/** Signal tier 1–4 for setup Wi‑Fi scan list badges (higher = stronger). */
export function setupWifiSignalTier(dbm: number): number {
  if (dbm >= -50) return 4;
  if (dbm >= -60) return 3;
  if (dbm >= -70) return 2;
  return 1;
}
