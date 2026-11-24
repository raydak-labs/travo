import { describe, expect, it } from 'vitest';
import { formatBitRate, formatBytes, formatLoad, formatUptime } from './format';
import { m } from './paraglide/messages.js';

describe('formatUptime', () => {
  it.each([
    [0, '<1m'],
    [59, '<1m'],
    [60, '1m'],
    [3600, '1h'],
    [3660, '1h 1m'],
    [90061, '1d 1h'],
    [86400, '1d'],
    [86400 + 60, '1d 1m'],
  ])('%i s -> %s', (s, out) => expect(formatUptime(s)).toBe(out));
});

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [1023, '1023 B'],
    [1024, '1.0 KB'],
    [428646400, '408.8 MB'],
    [81616896, '77.8 MB'],
    [3 * 1024 ** 3, '3.0 GB'],
  ])('%i -> %s', (n, out) => expect(formatBytes(n)).toBe(out));
});

describe('formatLoad', () => {
  it('pads to two decimals', () =>
    expect(formatLoad([0.39, 0.24, 0.1])).toBe('0.39 · 0.24 · 0.10'));
});

describe('formatBitRate', () => {
  it.each([
    [0, '0 bit/s'],
    [85, '85 bit/s'],
    [99, '99 bit/s'],
    [99.6, '0.1 kbit/s'],
    [850, '0.9 kbit/s'],
    [9_940, '9.9 kbit/s'],
    [12_000, '12 kbit/s'],
    [99_400, '99 kbit/s'],
    [99_600, '0.1 Mbit/s'],
    [850_000, '0.9 Mbit/s'],
    [1_200_000, '1.2 Mbit/s'],
    [12_000_000, '12 Mbit/s'],
    [1_500_000_000, '1.5 Gbit/s'],
    [250_000_000_000, '250 Gbit/s'],
  ])('%d -> %s', (bits, out) => expect(formatBitRate(bits)).toBe(out));

  it.each([null, -1, Number.NaN, Number.POSITIVE_INFINITY])('%s is unknown', (bits) => {
    expect(formatBitRate(bits)).toBe(m.dashboard_rate_unknown());
  });
});
