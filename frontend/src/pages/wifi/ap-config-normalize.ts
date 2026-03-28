import { wifiApEncryptionEnum, type APRadioFormValues } from '@/lib/schemas/wifi-forms';

export function normalizeApEncryption(v: string | undefined): APRadioFormValues['encryption'] {
  return wifiApEncryptionEnum.safeParse(v).success ? (v as APRadioFormValues['encryption']) : 'psk2';
}
