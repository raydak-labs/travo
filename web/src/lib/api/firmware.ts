import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type FirmwareImage,
  type FirmwareStatus,
  getFirmware,
  postFirmware,
  postFirmwareApply,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

/** The phrase the daemon requires, typed exactly. */
export const FIRMWARE_PHRASE = 'upgrade firmware';

export function phraseMatches(typed: string): boolean {
  return typed === FIRMWARE_PHRASE;
}

export async function fetchFirmware(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<FirmwareStatus> {
  const { data, response } = await getFirmware({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.firmware_unavailable(), response?.status);
  return data;
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function firmwareError(code: string | undefined): string {
  switch (code) {
    case 'firmware_too_large':
      return m.firmware_err_too_large();
    case 'firmware_no_room':
      return m.firmware_err_no_room();
    case 'firmware_upgrade_pending':
      return m.firmware_err_pending();
    case 'firmware_upload_busy':
      return m.firmware_err_upload_busy();
    case 'operation_in_progress':
      return m.firmware_err_busy();
    case 'firmware_not_staged':
      return m.firmware_err_not_staged();
    case 'firmware_invalid':
      return m.firmware_err_invalid();
    case 'firmware_no_backup':
      return m.firmware_err_no_backup();
    case 'confirmation_required':
      return m.firmware_err_phrase();
    case 'session_required':
      return m.firmware_err_session();
    default:
      return m.firmware_err_failed();
  }
}

/** True when the image is for this router (fwtool's board check passed). */
export function boardMatches(image: FirmwareImage): boolean {
  return image.validation.tests.fwtool_device_match === true;
}

export async function uploadFirmware(
  file: File,
): Promise<{ image: FirmwareImage } | { error: string }> {
  try {
    const { data, error } = await postFirmware({ body: { image: file } });
    if (data) return { image: data };
    return { error: firmwareError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: firmwareError(undefined) };
  }
}

export async function applyFirmware(
  sha256: string,
  keepSettings: boolean,
  typed: string,
): Promise<{ ok: true } | { error: string }> {
  try {
    const { data, error } = await postFirmwareApply({
      body: { sha256, keep_settings: keepSettings, confirm: typed },
    });
    if (data) return { ok: true };
    return { error: firmwareError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: firmwareError(undefined) };
  }
}
