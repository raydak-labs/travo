import type { GuestWifiFormValues } from '@/lib/schemas/wifi-forms';

export const GUEST_ENCRYPTION_VALUES = ['none', 'psk2', 'sae', 'psk-mixed'] as const;

export function normalizeGuestEncryption(v: string | undefined): GuestWifiFormValues['encryption'] {
  return GUEST_ENCRYPTION_VALUES.includes(v as GuestWifiFormValues['encryption'])
    ? (v as GuestWifiFormValues['encryption'])
    : 'psk2';
}
