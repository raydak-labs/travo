import type { Result } from './api/gen';
import { m } from './paraglide/messages.js';

// The schema allows null, but the generated enum type drops it (as for `medium`).
type Reason = Result['disabled_reason'] | null;

/** What the Updates page says when checks are disabled, by the daemon's reason. */
export function disabledMessage(result: {
  disabled_reason: Reason;
  running_version: Result['running_version'];
}): string {
  switch (result.disabled_reason) {
    case 'off':
      return m.update_disabled_off();
    case 'unstable_build':
      return m.update_disabled_unstable({ version: result.running_version });
    default:
      return m.update_disabled();
  }
}
