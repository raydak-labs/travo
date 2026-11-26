import type { AccessPoint } from './api/gen';
import { m } from './paraglide/messages.js';

export function bandLabel(band: string): string {
  switch (band) {
    case '2g':
      return '2.4 GHz';
    case '5g':
      return '5 GHz';
    case '6g':
      return '6 GHz';
    default:
      return m.wifi_band_unknown();
  }
}

export function bandsBadge(bands: string[]): string {
  if (bands.length === 0) return '';
  return `${bands.map((band) => bandLabel(band).replace(' GHz', '')).join(' + ')} GHz`;
}

export function signalLabel(signal: number): string {
  return `${signal} dBm`;
}

export function accessPointKey(accessPoint: AccessPoint, index: number): string {
  return `${accessPoint.band}:${index}:${accessPoint.bssid}`;
}

export function securityLabel(security: AccessPoint['security']): string {
  switch (security) {
    case 'open':
      return m.wifi_security_open();
    case 'wpa2':
      return m.wifi_security_wpa2();
    case 'wpa2_wpa3':
      return m.wifi_security_wpa2_wpa3();
    default:
      return m.wifi_security_other();
  }
}

export function wifiScanErrorMessage(error: unknown): string {
  const code =
    typeof error === 'object' && error !== null && 'code' in error
      ? (error as { code?: unknown }).code
      : undefined;
  if (code === 'wifi_scan_failed') return m.wifi_scan_failed();
  if (code === 'wifi_unavailable') return m.wifi_unavailable();
  return error instanceof Error && error.message ? error.message : m.wifi_unavailable();
}
