import { describe, expect, it } from 'vitest';
import { formFrom, parseDNS, requestFrom, wanRefusal } from './wan';

const base = { ipaddr: '', gateway: '', dns: '', username: '', password: '' };

describe('wan request', () => {
  it('sends only the chosen protocol fields', () => {
    expect(
      requestFrom({ ...base, proto: 'dhcp', ipaddr: '203.0.113.5/24', username: 'u' }),
    ).toEqual({
      proto: 'dhcp',
    });
    expect(
      requestFrom({
        ...base,
        proto: 'static',
        ipaddr: ' 203.0.113.5/24 ',
        gateway: '203.0.113.1',
        dns: '1.1.1.1, 9.9.9.9\n',
        username: 'stale',
      }),
    ).toEqual({
      proto: 'static',
      ipaddr: '203.0.113.5/24',
      gateway: '203.0.113.1',
      dns: ['1.1.1.1', '9.9.9.9'],
    });
  });

  it('omits an empty PPPoE password so the stored one is kept', () => {
    expect(requestFrom({ ...base, proto: 'pppoe', username: 'u@isp' })).toEqual({
      proto: 'pppoe',
      username: 'u@isp',
    });
    expect(requestFrom({ ...base, proto: 'pppoe', username: 'u@isp', password: ' p ' })).toEqual({
      proto: 'pppoe',
      username: 'u@isp',
      password: ' p ',
    });
  });

  it('fills the form without a password', () => {
    const form = formFrom({
      proto: 'static',
      ipaddr: '203.0.113.5/24',
      gateway: '203.0.113.1',
      dns: ['1.1.1.1', '9.9.9.9'],
      username: '',
      password_set: false,
      device: 'wan',
    });
    expect(form).toEqual({
      ...base,
      proto: 'static',
      ipaddr: '203.0.113.5/24',
      gateway: '203.0.113.1',
      dns: '1.1.1.1, 9.9.9.9',
    });
    expect(parseDNS(' , ')).toEqual([]);
  });

  it('maps refusal codes', () => {
    expect(wanRefusal('wan_no_change').message).toBe('The WAN already has these settings.');
    expect(wanRefusal('wan_overlaps_lan').error).toContain('overlaps the LAN');
    expect(wanRefusal(undefined).error).toBe('Could not change the WAN settings.');
  });
});
