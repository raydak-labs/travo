/** Map scan encryption names to UCI encryption values */
export function mapScanEncryptionToUci(enc: string): string {
  switch (enc) {
    case 'wpa2':
      return 'psk2';
    case 'wpa3':
      return 'sae';
    case 'wpa2/wpa3':
      return 'psk-mixed';
    case 'wpa':
      return 'psk';
    case 'none':
      return 'none';
    default:
      return 'psk2';
  }
}
