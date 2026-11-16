import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import PasswordField from './PasswordField.svelte';

describe('PasswordField contract', () => {
  it('associates the label and field error while retaining password-manager attributes', () => {
    const { body } = render(PasswordField, {
      props: {
        id: 'new-password',
        label: 'New password',
        autocomplete: 'new-password',
        error: 'Too short',
      },
    });
    expect(body).toMatch(/<label[^>]*for="new-password"/);
    expect(body).toMatch(/<input[^>]*type="password"[^>]*name="new-password"/);
    expect(body).toContain('autocomplete="new-password"');
    expect(body).toContain('required=');
    expect(body).toContain('aria-invalid="true"');
    expect(body).toContain('aria-describedby="new-password-error"');
    expect(body).toMatch(/<p[^>]*id="new-password-error"[^>]*>Too short<\/p>/);
  });

  it('does not expose an error relationship when no field error exists', () => {
    const { body } = render(PasswordField, {
      props: { id: 'current-password', label: 'Password', autocomplete: 'current-password' },
    });
    expect(body).not.toContain('aria-invalid=');
    expect(body).not.toContain('aria-describedby=');
    expect(body).toContain('autocomplete="current-password"');
  });
});
