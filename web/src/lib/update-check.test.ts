import { describe, expect, it } from 'vitest';
import { m } from './paraglide/messages.js';
import { disabledMessage, showOperation } from './update-check';

describe('disabledMessage', () => {
  it('says plainly that update checks are not set up', () => {
    expect(disabledMessage({ disabled_reason: 'off', running_version: '1.2.3' })).toBe(
      m.update_disabled_off(),
    );
    expect(m.update_disabled_off()).toContain('not set up');
  });

  it('names the development build', () => {
    expect(
      disabledMessage({ disabled_reason: 'unstable_build', running_version: 'dev' }),
    ).toContain('dev');
  });

  it('keeps the generic text without a reason', () => {
    expect(disabledMessage({ disabled_reason: null, running_version: '1.2.3' })).toBe(
      m.update_disabled(),
    );
  });
});

describe('showOperation', () => {
  it('hides an old succeeded journal after a later install', () => {
    expect(showOperation({ state: 'succeeded', version: '0.1.0' }, '0.4.0', false)).toBe(false);
  });
  it('shows a succeeded update that matches what runs, or one this page started', () => {
    expect(showOperation({ state: 'succeeded', version: '0.4.0' }, '0.4.0', false)).toBe(true);
    expect(showOperation({ state: 'rolled_back', version: '0.4.0' }, '0.3.0', true)).toBe(true);
  });
  it('always shows an apply in progress and a needed recovery', () => {
    expect(showOperation({ state: 'applying', version: '0.4.0' }, '0.3.0', false)).toBe(true);
    expect(showOperation({ state: 'recovery_required' }, '0.3.0', false)).toBe(true);
    expect(showOperation({ state: 'aborted', version: '0.4.0' }, '0.3.0', false)).toBe(false);
    expect(showOperation(undefined, '0.3.0', true)).toBe(false);
  });
});
