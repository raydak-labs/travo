import { describe, it, expect } from 'vitest';
import { encodePathSegment, routeWithParam, routeWithSegment } from '../api-url';

describe('encodePathSegment', () => {
  it('escapes characters that would change the route structure', () => {
    expect(encodePathSegment('a/b')).toBe('a%2Fb');
    expect(encodePathSegment('../../system/reboot')).toBe('..%2F..%2Fsystem%2Freboot');
    expect(encodePathSegment('guest wifi')).toBe('guest%20wifi');
    expect(encodePathSegment(3)).toBe('3');
  });
});

describe('routeWithParam', () => {
  it('fills and encodes a template placeholder', () => {
    expect(routeWithParam('/api/v1/services/:id/install', 'tailscale')).toBe(
      '/api/v1/services/tailscale/install',
    );
    expect(routeWithParam('/api/v1/services/:id/install', '../../x')).toBe(
      '/api/v1/services/..%2F..%2Fx/install',
    );
    expect(routeWithParam('/api/v1/wifi/radios/:name/role', 'phy1')).toBe(
      '/api/v1/wifi/radios/phy1/role',
    );
  });
});

describe('routeWithSegment', () => {
  it('appends an encoded segment', () => {
    expect(routeWithSegment('/api/v1/system/ssh-keys', 2)).toBe('/api/v1/system/ssh-keys/2');
    expect(routeWithSegment('/api/v1/wifi/saved', 'a b')).toBe('/api/v1/wifi/saved/a%20b');
  });
});
