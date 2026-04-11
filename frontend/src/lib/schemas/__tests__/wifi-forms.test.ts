import { describe, it, expect } from 'vitest';
import {
  createWifiConnectFormSchema,
  wifiHiddenNetworkFormSchema,
  macPolicyAddFormSchema,
  wifiScheduleFormSchema,
  guestWifiFormSchema,
  unifiedApCredentialsSchema,
  macCloneFormSchema,
} from '../wifi-forms';

describe('createWifiConnectFormSchema', () => {
  it('allows empty password when open network', () => {
    const s = createWifiConnectFormSchema(false);
    const r = s.safeParse({ password: '', selectedBand: '5ghz' });
    expect(r.success).toBe(true);
  });

  it('requires min 8 chars when encrypted', () => {
    const s = createWifiConnectFormSchema(true);
    expect(s.safeParse({ password: 'short', selectedBand: '' }).success).toBe(false);
    expect(s.safeParse({ password: 'longenough', selectedBand: '' }).success).toBe(true);
  });
});

describe('wifiHiddenNetworkFormSchema', () => {
  it('requires SSID', () => {
    const r = wifiHiddenNetworkFormSchema.safeParse({
      ssid: '   ',
      encryption: 'psk2',
      password: 'password1',
    });
    expect(r.success).toBe(false);
  });

  it('allows open network without password', () => {
    const r = wifiHiddenNetworkFormSchema.safeParse({
      ssid: 'OpenHidden',
      encryption: 'none',
      password: '',
    });
    expect(r.success).toBe(true);
  });

  it('requires password for WPA2', () => {
    const r = wifiHiddenNetworkFormSchema.safeParse({
      ssid: 'Secured',
      encryption: 'psk2',
      password: '',
    });
    expect(r.success).toBe(false);
  });
});

describe('macPolicyAddFormSchema', () => {
  it('allows empty MAC', () => {
    const r = macPolicyAddFormSchema.safeParse({ ssid: 'MyNet', mac: '' });
    expect(r.success).toBe(true);
  });

  it('rejects bad MAC when set', () => {
    const r = macPolicyAddFormSchema.safeParse({ ssid: 'MyNet', mac: 'not-a-mac' });
    expect(r.success).toBe(false);
  });
});

describe('wifiScheduleFormSchema', () => {
  it('accepts valid times', () => {
    const r = wifiScheduleFormSchema.safeParse({
      enabled: true,
      on_time: '08:30',
      off_time: '22:00',
    });
    expect(r.success).toBe(true);
  });

  it('rejects invalid time', () => {
    const r = wifiScheduleFormSchema.safeParse({
      enabled: true,
      on_time: '25:00',
      off_time: '22:00',
    });
    expect(r.success).toBe(false);
  });
});

describe('guestWifiFormSchema', () => {
  it('skips SSID check when disabled', () => {
    const r = guestWifiFormSchema.safeParse({
      enabled: false,
      ssid: '',
      encryption: 'psk2',
      key: '',
    });
    expect(r.success).toBe(true);
  });

  it('requires SSID when enabled', () => {
    const r = guestWifiFormSchema.safeParse({
      enabled: true,
      ssid: ' ',
      encryption: 'psk2',
      key: 'password1',
    });
    expect(r.success).toBe(false);
  });
});

describe('unifiedApCredentialsSchema', () => {
  it('requires SSID and WPA key length', () => {
    expect(
      unifiedApCredentialsSchema.safeParse({ ssid: '', encryption: 'psk2', key: 'x' }).success,
    ).toBe(false);
    expect(
      unifiedApCredentialsSchema.safeParse({
        ssid: 'Net',
        encryption: 'psk2',
        key: 'longenough',
      }).success,
    ).toBe(true);
  });

  it('allows open network without password', () => {
    const r = unifiedApCredentialsSchema.safeParse({
      ssid: 'Open',
      encryption: 'none',
      key: '',
    });
    expect(r.success).toBe(true);
  });
});

describe('macCloneFormSchema', () => {
  it('allows empty MAC', () => {
    const r = macCloneFormSchema.safeParse({ custom_mac: '' });
    expect(r.success).toBe(true);
  });

  it('rejects invalid MAC', () => {
    const r = macCloneFormSchema.safeParse({ custom_mac: 'bad' });
    expect(r.success).toBe(false);
  });
});
