const MIN_BYTES = 8;
const MAX_BYTES = 128;

/**
 * Same rules as the daemon (`auth.ValidatePassword`): 8 to 128 UTF-8 bytes, no NUL or line breaks.
 * The daemon stays the authority; this only saves a round trip.
 */
export function passwordRulesOk(password: string): boolean {
  const bytes = new TextEncoder().encode(password).length;
  return bytes >= MIN_BYTES && bytes <= MAX_BYTES && !/[\0\n\r]/.test(password);
}
