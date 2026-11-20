import './api/client';
import { client } from './api/client';
import { getUsbTether, installUsbTetherDrivers, type UsbTetherStatus } from './api/gen';
import type { Client } from './api/gen/client';
import { APIRequestError } from './api/request';
import { m } from './paraglide/messages.js';

export type Phone = 'android' | 'ios';

export async function fetchUsbTether(signal?: AbortSignal, requestClient: Client = client) {
  const { data, response } = await getUsbTether({ signal, client: requestClient });
  if (!data) {
    throw new APIRequestError(m.usbtether_unavailable(), response?.status);
  }
  return data;
}

/** Installs a driver set; the result is the message to show, never a thrown error. */
export async function installDrivers(
  phone: Phone,
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<{ message: string } | { error: string }> {
  const { data, error } = await installUsbTetherDrivers({
    body: { phone },
    signal,
    client: requestClient,
  });
  if (data) {
    return {
      message: data.installed ? m.usbtether_drivers_installed() : m.usbtether_drivers_present(),
    };
  }
  switch ((error as { code?: string } | undefined)?.code) {
    case 'usbtether_no_space':
      return { error: m.usbtether_drivers_no_space() };
    case 'usbtether_install_busy':
      return { error: m.usbtether_drivers_busy() };
    default:
      return { error: m.usbtether_drivers_failed() };
  }
}

/** Maps a refused POST /usbtether to its message. */
export function usbTetherRefusal(code: string | undefined): { message?: string; error?: string } {
  switch (code) {
    case 'usbtether_no_device':
      return { error: m.usbtether_no_device() };
    case 'usbtether_ambiguous':
      return { error: m.usbtether_ambiguous() };
    case 'usbtether_unsupported':
      return { error: m.usbtether_unsupported() };
    case 'usbtether_no_change':
      return { message: m.usbtether_no_change() };
    case 'transaction_pending':
      return { error: m.vpn_tunnel_busy() };
    default:
      return { error: m.usbtether_failed() };
  }
}

/** Driver state of one phone type: installed only when every package is; unknown wins over absent. */
export function driverState(
  status: UsbTetherStatus,
  phone: Phone,
): 'installed' | 'absent' | 'unknown' {
  const states = status.drivers.filter((d) => d.phone === phone).map((d) => d.state);
  if (states.includes('unknown')) return 'unknown';
  return states.length > 0 && states.every((s) => s === 'installed') ? 'installed' : 'absent';
}

/** The bound link is gone: the phone was unplugged or came back under another name. */
export function boundLinkMissing(status: UsbTetherStatus): boolean {
  return status.configured && !status.links.some((l) => l.name === status.device);
}

/**
 * The iPhone link an enable targets has no carrier: the phone is locked, does not trust the
 * router yet, or Personal Hotspot is off. Live, the first enable rolled back for this reason.
 */
export function iphoneNotReady(status: UsbTetherStatus | undefined, device: string): boolean {
  const link = status?.links.find((l) => l.name === device);
  return link?.kind === 'ipheth' && link.carrier === false;
}

/** Adds the iPhone hint to a refusal or rollback toast when iphoneNotReady explains it. */
export function withIphoneHint(
  text: string,
  status: UsbTetherStatus | undefined,
  device: string,
): string {
  return iphoneNotReady(status, device) ? `${text} ${m.usbtether_iphone_hint()}` : text;
}
