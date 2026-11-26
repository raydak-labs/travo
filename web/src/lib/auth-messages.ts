import { m } from './paraglide/messages.js';

/** What a page shows for a failed auth call. `field` names the input the text belongs to. */
export type AuthMessage = {
  text: string;
  field: 'password' | 'current' | 'new' | null;
  /** Follow-up the page offers next to the text. */
  action?: 'login' | 'setup' | 'retry';
};

const unavailable = (): AuthMessage => ({
  text: m.device_unavailable(),
  field: null,
  action: 'retry',
});
const rateLimited = (): AuthMessage => ({ text: m.rate_limited(), field: null });

// Unknown codes fall back to "unavailable + retry": the user can always try again, and the
// daemon's message text is not for branching or display (invariants §4).

export function setupMessage(code: string): AuthMessage {
  switch (code) {
    case 'already_set':
      return { text: m.setup_already_set(), field: null, action: 'login' };
    case 'invalid_password':
    case 'invalid_request':
      return { text: m.password_rules(), field: 'password' };
    case 'rate_limited':
      return rateLimited();
    default:
      return unavailable();
  }
}

export function loginMessage(code: string): AuthMessage {
  switch (code) {
    case 'invalid_credentials':
      return { text: m.login_wrong_password(), field: 'password' };
    case 'setup_required':
      return { text: m.setup_title(), field: null, action: 'setup' };
    case 'rate_limited':
      return rateLimited();
    default:
      return unavailable();
  }
}

export function passwordMessage(code: string): AuthMessage {
  switch (code) {
    case 'invalid_credentials':
      return { text: m.password_wrong_current(), field: 'current' };
    case 'invalid_password':
      return { text: m.password_rules(), field: 'new' };
    case 'rate_limited':
      return rateLimited();
    default:
      return unavailable();
  }
}
