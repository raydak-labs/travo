import { describe, expect, it } from 'vitest';
import { loginMessage, passwordMessage, setupMessage } from './auth-messages';
import { m } from './paraglide/messages.js';

const unavailable = { text: m.device_unavailable(), field: null, action: 'retry' };
const limited = { text: m.rate_limited(), field: null };

describe('setupMessage', () => {
  it.each([
    ['already_set', { text: m.setup_already_set(), field: null, action: 'login' }],
    ['invalid_password', { text: m.password_rules(), field: 'password' }],
    ['invalid_request', { text: m.password_rules(), field: 'password' }],
    ['rate_limited', limited],
    ['device_unavailable', unavailable],
    ['network', unavailable],
    ['something_new', unavailable],
  ])('%s', (code, want) => {
    expect(setupMessage(code)).toEqual(want);
  });
});

describe('loginMessage', () => {
  it.each([
    ['invalid_credentials', { text: m.login_wrong_password(), field: 'password' }],
    ['setup_required', { text: m.setup_title(), field: null, action: 'setup' }],
    ['rate_limited', limited],
    ['device_unavailable', unavailable],
    ['network', unavailable],
    ['something_new', unavailable],
  ])('%s', (code, want) => {
    expect(loginMessage(code)).toEqual(want);
  });
});

describe('passwordMessage', () => {
  it.each([
    ['invalid_credentials', { text: m.password_wrong_current(), field: 'current' }],
    ['invalid_password', { text: m.password_rules(), field: 'new' }],
    ['rate_limited', limited],
    ['persist_failed', unavailable],
    ['network', unavailable],
  ])('%s', (code, want) => {
    expect(passwordMessage(code)).toEqual(want);
  });
});
