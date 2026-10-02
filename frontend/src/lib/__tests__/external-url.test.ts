import { describe, it, expect, vi, afterEach } from 'vitest';
import { isSafeExternalUrl, openExternalUrl } from '../external-url';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('isSafeExternalUrl', () => {
  it('accepts absolute http and https URLs', () => {
    expect(isSafeExternalUrl('http://captive.apple.com/hotspot.html')).toBe(true);
    expect(isSafeExternalUrl('https://login.tailscale.com/a/abc123')).toBe(true);
  });

  it('rejects script-bearing and non-web schemes', () => {
    expect(isSafeExternalUrl('javascript:alert(document.cookie)')).toBe(false);
    expect(isSafeExternalUrl('JavaScript:alert(1)')).toBe(false);
    expect(isSafeExternalUrl('data:text/html,<script>alert(1)</script>')).toBe(false);
    expect(isSafeExternalUrl('file:///etc/passwd')).toBe(false);
  });

  it('rejects relative and empty values', () => {
    expect(isSafeExternalUrl('/setup')).toBe(false);
    expect(isSafeExternalUrl('login.tailscale.com')).toBe(false);
    expect(isSafeExternalUrl('')).toBe(false);
    expect(isSafeExternalUrl(null)).toBe(false);
    expect(isSafeExternalUrl(undefined)).toBe(false);
  });
});

describe('openExternalUrl', () => {
  it('opens allowed URLs with the opener isolated', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);

    expect(openExternalUrl('https://login.tailscale.com/a/abc')).toBe(true);
    expect(open).toHaveBeenCalledWith(
      'https://login.tailscale.com/a/abc',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('opens nothing for a rejected URL', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);

    expect(openExternalUrl('javascript:alert(1)')).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });
});
