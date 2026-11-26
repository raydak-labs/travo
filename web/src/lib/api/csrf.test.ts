import { afterEach, describe, expect, it } from 'vitest';
import { csrf, csrfInterceptor } from './csrf';

const req = (method: string) => new Request('http://x.test/api/y', { method });

afterEach(() => csrf.set(null));

describe('csrfInterceptor', () => {
  it.each(['POST', 'PUT', 'PATCH', 'DELETE'])('adds X-CSRF-Token on %s', (method) => {
    csrf.set('tok');
    expect(csrfInterceptor(req(method)).headers.get('X-CSRF-Token')).toBe('tok');
  });

  it.each(['GET', 'HEAD', 'OPTIONS'])('does not add it on %s', (method) => {
    csrf.set('tok');
    expect(csrfInterceptor(req(method)).headers.has('X-CSRF-Token')).toBe(false);
  });

  it('adds nothing when no token is held', () => {
    expect(csrfInterceptor(req('POST')).headers.has('X-CSRF-Token')).toBe(false);
  });
});
