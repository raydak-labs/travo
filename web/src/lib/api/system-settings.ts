import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getSystemSettings, getTimezones, type SystemSettingsBody, setSystemSettings } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type DeviceSettings = { settings: SystemSettingsBody; zonenames: string[] };

/** Settings and the zone list load together: the form needs both. */
export async function fetchDeviceSettings(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DeviceSettings> {
  const [settings, zones] = await Promise.all([
    getSystemSettings({ signal, client: requestClient }),
    getTimezones({ signal, client: requestClient }),
  ]);
  if (!settings.data)
    throw new APIRequestError(m.settings_unavailable(), settings.response?.status);
  if (!zones.data) throw new APIRequestError(m.settings_unavailable(), zones.response?.status);
  return { settings: settings.data, zonenames: zones.data.zonenames };
}

/** One server per line, comma or space; blanks dropped, order kept. */
export function parseServers(text: string): string[] {
  return text
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter((s) => s !== '');
}

/** The router's own name: one DNS label (same rule as the daemon). */
export function validHostname(name: string): boolean {
  return /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$/.test(name);
}

export function settingsError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.settings_err_invalid();
    case 'uci_busy':
      return m.settings_err_busy();
    default:
      return m.settings_err_failed();
  }
}

/**
 * Saves the form. A changed zone sends an empty timezone so the daemon derives it; an
 * unchanged zone sends the stored string back so nothing is rewritten.
 */
export async function saveDeviceSettings(
  current: SystemSettingsBody,
  form: { hostname: string; zonename: string; ntpEnabled: boolean; servers: string },
): Promise<{ settings: SystemSettingsBody } | { error: string }> {
  const body: SystemSettingsBody = {
    hostname: form.hostname.trim(),
    zonename: form.zonename,
    timezone: form.zonename === current.zonename ? current.timezone : '',
    ntp: { enabled: form.ntpEnabled, servers: parseServers(form.servers) },
  };
  try {
    const { data, error } = await setSystemSettings({ body });
    if (data) return { settings: data };
    return { error: settingsError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: settingsError(undefined) };
  }
}
