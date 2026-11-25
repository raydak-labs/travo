import { describe, expect, it } from 'vitest';
import { m } from './paraglide/messages.js';
import { disabledMessage } from './update-check';

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
