import { describe, expect, it } from 'vitest';
import { passwordRulesOk } from './password-rules';

describe('passwordRulesOk', () => {
  it.each([
    ['min', '12345678', true],
    ['short', '1234567', false],
    ['max', 'a'.repeat(128), true],
    ['too long', 'a'.repeat(129), false],
    ['nul', 'abcdefg\0h', false],
    ['newline', 'abcdefg\nh', false],
    ['carriage return', 'abcdefg\rh', false],
    ['counts bytes, not characters', 'äääää', true], // 5 characters, 10 bytes
    ['too long in bytes', 'ä'.repeat(65), false], // 65 characters, 130 bytes
    ['spaces ok', 'pass word!', true],
  ])('%s', (_name, password, ok) => {
    expect(passwordRulesOk(password)).toBe(ok);
  });
});
