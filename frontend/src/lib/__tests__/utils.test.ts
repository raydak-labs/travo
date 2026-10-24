import { describe, it, expect } from 'vitest';
import { formatBitRate, formatBytes, formatUptime } from '../utils';

describe('formatBytes', () => {
  it('returns "0 B" for 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('returns bytes for values under 1024', () => {
    expect(formatBytes(500)).toBe('500 B');
  });

  it('handles exactly 1 KB', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
  });

  it('handles 1 MB', () => {
    expect(formatBytes(1048576)).toBe('1.0 MB');
  });

  it('handles 1 GB', () => {
    expect(formatBytes(1073741824)).toBe('1.0 GB');
  });

  it('handles 1 TB', () => {
    expect(formatBytes(1099511627776)).toBe('1.0 TB');
  });

  it('formats fractional values', () => {
    expect(formatBytes(1536)).toBe('1.5 KB');
  });

  it('handles negative values gracefully', () => {
    // negative bytes should return a negative formatted value or handle gracefully
    const result = formatBytes(-1);
    expect(typeof result).toBe('string');
  });
});

describe('formatBitRate', () => {
  it('keeps a low bit rate in Mbit instead of growing a third digit', () => {
    expect(formatBitRate(850_000)).toBe('0.9 Mbit/s');
  });

  it('formats a normal link rate with one decimal', () => {
    expect(formatBitRate(25_000_000)).toBe('25.0 Mbit/s');
  });

  it('moves to Gbit at 100 Mbit so the integer part stays short', () => {
    expect(formatBitRate(100_000_000)).toBe('0.1 Gbit/s');
    expect(formatBitRate(940_000_000)).toBe('0.9 Gbit/s');
    expect(formatBitRate(100_000_000_000)).toBe('0.1 Tbit/s');
  });

  it('rounds up across the unit boundary', () => {
    expect(formatBitRate(99_960_000)).toBe('0.1 Gbit/s');
  });

  it('marks rates below the display precision', () => {
    expect(formatBitRate(40_000)).toBe('<0.1 Mbit/s');
    expect(formatBitRate(0)).toBe('0.0 Mbit/s');
  });

  it('rejects non-finite samples', () => {
    expect(formatBitRate(Number.NaN)).toBe('0.0 Mbit/s');
    expect(formatBitRate(Number.POSITIVE_INFINITY)).toBe('0.0 Mbit/s');
  });
});

describe('formatUptime', () => {
  it('returns "0 minutes" for 0 seconds', () => {
    expect(formatUptime(0)).toBe('0 minutes');
  });

  it('formats seconds less than a minute', () => {
    expect(formatUptime(30)).toBe('0 minutes');
  });

  it('formats exactly 1 minute', () => {
    expect(formatUptime(60)).toBe('1 minute');
  });

  it('formats multiple minutes', () => {
    expect(formatUptime(120)).toBe('2 minutes');
  });

  it('formats 1 hour', () => {
    expect(formatUptime(3600)).toBe('1 hour');
  });

  it('formats hours and minutes', () => {
    expect(formatUptime(3660)).toBe('1 hour, 1 minute');
  });

  it('formats 1 day', () => {
    expect(formatUptime(86400)).toBe('1 day');
  });

  it('formats days, hours, and minutes', () => {
    expect(formatUptime(90061)).toBe('1 day, 1 hour, 1 minute');
  });

  it('formats the mock uptime value 86432', () => {
    // 86432 = 1 day + 32 seconds = 1 day
    expect(formatUptime(86432)).toBe('1 day');
  });

  it('formats large values', () => {
    // 7 days, 3 hours, 25 minutes = 7*86400 + 3*3600 + 25*60 = 617100
    expect(formatUptime(617100)).toBe('7 days, 3 hours, 25 minutes');
  });
});
