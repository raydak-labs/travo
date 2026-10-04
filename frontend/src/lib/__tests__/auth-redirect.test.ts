import { describe, expect, it } from 'vitest';
import { currentRelativeLocation, safeRedirectTarget } from '@/lib/auth-redirect';

describe('currentRelativeLocation', () => {
  it('captures pathname and query so a deep link survives the login bounce', () => {
    window.history.replaceState(null, '', '/wifi/advanced?tab=ap');
    expect(currentRelativeLocation()).toBe('/wifi/advanced?tab=ap');
  });
});

describe('safeRedirectTarget', () => {
  it('keeps an in-app relative path', () => {
    expect(safeRedirectTarget('/network/configuration')).toBe('/network/configuration');
  });

  it.each([
    ['undefined', undefined],
    ['empty', ''],
    ['protocol-relative URL', '//evil.example/steal'],
    ['absolute URL', 'https://evil.example/steal'],
    ['non-path', 'javascript:alert(1)'],
    ['the login page itself', '/login'],
  ])('falls back to the dashboard for %s', (_label, value) => {
    expect(safeRedirectTarget(value as string | undefined)).toBe('/dashboard');
  });

  it('falls back when the login page carries a query', () => {
    expect(safeRedirectTarget('/login?redirect=/logs')).toBe('/dashboard');
  });
});
