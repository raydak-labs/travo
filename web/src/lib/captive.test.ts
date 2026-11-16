import { describe, expect, it } from 'vitest';
import { captiveError, formatCountdown } from './captive';

describe('captive helpers', () => {
  it('formats the relative restore countdown', () => {
    expect(formatCountdown(300)).toBe('5:00');
    expect(formatCountdown(61)).toBe('1:01');
    expect(formatCountdown(0)).toBe('0:00');
    expect(formatCountdown(-3)).toBe('0:00');
  });

  it('names the cause for each stable error code', () => {
    expect(captiveError('captive_no_uplink_dns')).toBe('The uplink did not provide a DNS server.');
    expect(captiveError('dns_foreign_staged')).toContain('another tool');
    expect(captiveError('dns_change_pending')).toContain('did not finish');
    expect(captiveError(undefined)).toBe('The DNS change failed.');
  });
});
