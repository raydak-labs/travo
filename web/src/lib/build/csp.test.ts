import { describe, expect, it } from 'vitest';
import { inlineScriptHash } from '../../../scripts/csp-hash.mjs';

describe('inlineScriptHash', () => {
  it('hashes the content of the single inline script', () => {
    expect(inlineScriptHash('<html><script>a</script></html>')).toBe(
      'sha256-ypeBEsobvcr6wjGzmiPcTaeG7/gUfE5yuYB3ha/uSLs=',
    );
  });

  it('ignores scripts with a src attribute', () => {
    const html = '<script type="module" src="/x.js"></script><script>a</script>';
    expect(inlineScriptHash(html)).toBe('sha256-ypeBEsobvcr6wjGzmiPcTaeG7/gUfE5yuYB3ha/uSLs=');
  });

  it('throws without an inline script', () => {
    expect(() => inlineScriptHash('<script src="/x.js"></script>')).toThrow(/exactly one/);
    expect(() => inlineScriptHash('<html></html>')).toThrow(/exactly one/);
  });

  it('throws with two inline scripts', () => {
    expect(() => inlineScriptHash('<script>a</script><script>b</script>')).toThrow(/exactly one/);
  });
});
