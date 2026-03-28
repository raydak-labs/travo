import { describe, expect, it } from 'vitest';
import { formatWifiBandLabel, normalizeWifiBandKey } from '../wifi-band';

describe('formatWifiBandLabel', () => {
  it('formats known bands', () => {
    expect(formatWifiBandLabel('5ghz')).toBe('5 GHz');
    expect(formatWifiBandLabel('5GHz')).toBe('5 GHz');
    expect(formatWifiBandLabel('2.4g')).toBe('2.4 GHz');
    expect(formatWifiBandLabel('6ghz')).toBe('6 GHz');
  });

  it('returns unknown strings unchanged', () => {
    expect(formatWifiBandLabel('custom')).toBe('custom');
  });
});

describe('normalizeWifiBandKey', () => {
  it('normalizes known bands', () => {
    expect(normalizeWifiBandKey('5GHz')).toBe('5ghz');
    expect(normalizeWifiBandKey('2.4g')).toBe('2.4ghz');
  });

  it('passes through other values', () => {
    expect(normalizeWifiBandKey('5 ghz')).toBe('5 ghz');
  });
});
