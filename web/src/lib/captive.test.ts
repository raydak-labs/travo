import { describe, expect, it } from 'vitest';
import { acceptError, acceptMessage, captiveError, formatCountdown } from './captive';

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

  it('reports each automatic login outcome as an expected answer', () => {
    expect(acceptMessage('online')).toEqual({
      kind: 'success',
      text: 'Logged in. The router is online.',
    });
    expect(acceptMessage('needs_user').kind).toBe('info');
    expect(acceptMessage('needs_user').text).toContain('room number');
    expect(acceptMessage('no_form').kind).toBe('error');
    expect(acceptMessage('failed').text).toContain('hotel DNS');
  });

  it('names the cause for each automatic login error code', () => {
    expect(acceptError('captive_accept_not_captive')).toBe('No hotel login is detected right now.');
    expect(acceptError('captive_accept_busy')).toContain('already running');
    expect(acceptError(undefined)).toContain('did not get online');
  });
});
