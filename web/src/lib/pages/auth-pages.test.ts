import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import Login from './Login.svelte';
import Setup from './Setup.svelte';

// Server render: labels, autocomplete and the submit button exist in the first paint.
describe('auth pages', () => {
  it('Login: labelled password input, current-password, submit', () => {
    const { body } = render(Login);
    expect(body).toContain('for="login-password"');
    expect(body).toContain('autocomplete="current-password"');
    expect(body).toContain('type="submit"');
    expect(body).not.toMatch(/<(?:input|button)\b[^>]*\sdisabled(?:=|\s|>)/);
  });

  it('Setup: two new-password inputs', () => {
    const { body } = render(Setup);
    expect(body).toContain('for="setup-password"');
    expect(body).toContain('for="setup-confirm"');
    expect(body.match(/autocomplete="new-password"/g)).toHaveLength(2);
  });

  it('Access: the change-password card (Log out is in the header menu)', async () => {
    const { default: PasswordCard } = await import('./PasswordCard.svelte');
    const { body } = render(PasswordCard);
    expect(body).toContain('for="password-current"');
    expect(body).toContain('autocomplete="current-password"');
    expect(body.match(/autocomplete="new-password"/g)).toHaveLength(2);
  });
});
