import type { WiFiProfileSummary } from './api/gen';
import { m } from './paraglide/messages.js';

/** The full ID order after moving one saved uplink by one place, or undefined at an end. */
export function moveProfile(
  profiles: WiFiProfileSummary[],
  index: number,
  delta: -1 | 1,
): string[] | undefined {
  const target = index + delta;
  if (index < 0 || target < 0 || target >= profiles.length) return undefined;
  return profiles.map((profile, i) => {
    if (i === index) return profiles[target]?.id ?? profile.id;
    if (i === target) return profiles[index]?.id ?? profile.id;
    return profile.id;
  });
}

export function saveError(code: string | undefined): string {
  switch (code) {
    case 'wifi_profile_exists':
      return m.wifi_save_exists();
    case 'wifi_profile_limit':
      return m.wifi_save_limit();
    case 'wifi_key_invalid':
    case 'wifi_key_required':
      return m.wifi_join_invalid_key();
    case 'wifi_security_unsupported':
      return m.wifi_join_unsupported();
    default:
      return m.wifi_save_failed();
  }
}

export function savedChangeError(code: string | undefined): string {
  switch (code) {
    case 'wifi_profile_not_found':
      return m.wifi_saved_not_found();
    case 'wifi_profile_order_invalid':
      return m.wifi_saved_order_changed();
    default:
      return m.wifi_saved_change_failed();
  }
}
