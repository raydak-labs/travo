import { m } from '../paraglide/messages.js';
import './client';
import { getWiFi, getWiFiScan, type JoinWiFiData, joinWiFi } from './gen';

export async function fetchWifi(signal?: AbortSignal) {
  const response = await getWiFi({ signal });
  if (!response.data) {
    const code = (response.error as { code?: string } | undefined)?.code;
    throw Object.assign(new Error(m.wifi_unavailable()), { code });
  }
  return response.data;
}

export async function fetchWifiScan(signal?: AbortSignal) {
  const response = await getWiFiScan({ signal });
  if (!response.data) {
    const code = (response.error as { code?: string } | undefined)?.code;
    throw Object.assign(new Error(m.wifi_unavailable()), { code });
  }
  return response.data;
}

export function joinWifi(body: JoinWiFiData['body'], signal?: AbortSignal) {
  return joinWiFi({ body, signal });
}
