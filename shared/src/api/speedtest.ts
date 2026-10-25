import type { SpeedtestService } from '../generated/model';

/** Generated as SpeedtestService. */
export type SpeedtestServiceStatus = SpeedtestService;

export type { SpeedTestResult } from './system';
export { isSpeedTestResult } from './system';

export function isSpeedtestServiceStatus(value: unknown): value is SpeedtestServiceStatus {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.installed === 'boolean' &&
    typeof v.supported === 'boolean' &&
    typeof v.architecture === 'string' &&
    typeof v.version === 'string' &&
    typeof v.package_name === 'string' &&
    typeof v.storage_size_mb === 'number'
  );
}
