import { m } from '../paraglide/messages.js';
import './client';
import {
  type CreateWiFiProfileData,
  createWiFiProfile,
  deleteWiFiProfile,
  getWiFi,
  getWiFiAccessPoints,
  getWiFiProfiles,
  getWiFiReconnect,
  getWiFiScan,
  type JoinWiFiData,
  joinWiFi,
  reorderWiFiProfiles,
  selectWiFiProfile,
  type UpdateWiFiAccessPointData,
  updateWiFiAccessPoint,
} from './gen';

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

export async function fetchWifiProfiles(signal?: AbortSignal) {
  const response = await getWiFiProfiles({ signal });
  if (!response.data) throw new Error(m.wifi_saved_unavailable());
  return response.data;
}

export async function fetchWifiReconnect(signal?: AbortSignal) {
  const response = await getWiFiReconnect({ signal });
  if (!response.data) throw new Error(m.wifi_reconnect_unavailable());
  return response.data;
}

export function saveWifiProfile(body: CreateWiFiProfileData['body'], signal?: AbortSignal) {
  return createWiFiProfile({ body, signal });
}

export function reorderWifiProfiles(ids: string[], signal?: AbortSignal) {
  return reorderWiFiProfiles({ body: { ids }, signal });
}

export function forgetWifiProfile(id: string, signal?: AbortSignal) {
  return deleteWiFiProfile({ path: { id }, signal });
}

export function selectWifiProfile(id: string, signal?: AbortSignal) {
  return selectWiFiProfile({ path: { id }, signal });
}

export async function fetchAccessPoints(signal?: AbortSignal) {
  const response = await getWiFiAccessPoints({ signal });
  if (!response.data) throw new Error(m.wifi_unavailable());
  return response.data.aps;
}

export function updateAccessPoint(
  section: string,
  body: UpdateWiFiAccessPointData['body'],
  signal?: AbortSignal,
) {
  return updateWiFiAccessPoint({ path: { section }, body, signal });
}
