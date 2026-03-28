import { describe, it, expect } from 'vitest';
import {
  wolFormSchema,
  dnsConfigFormSchema,
  ddnsFormSchema,
  diagnosticsFormSchema,
  dnsEntryFormSchema,
  dhcpReservationFormSchema,
  dhcpPoolFormSchema,
  normalizeDhcpLeaseTime,
  formatDhcpLeaseTimeHumanLabel,
  portForwardFormSchema,
  dataBudgetFormSchema,
} from '../network-forms';

describe('wolFormSchema', () => {
  it('accepts colon MAC', () => {
    const r = wolFormSchema.safeParse({ mac: 'AA:BB:CC:DD:EE:FF', interface: 'br-lan' });
    expect(r.success).toBe(true);
  });

  it('accepts hyphen MAC', () => {
    const r = wolFormSchema.safeParse({ mac: 'aa-bb-cc-dd-ee-ff', interface: '' });
    expect(r.success).toBe(true);
  });

  it('rejects invalid MAC', () => {
    const r = wolFormSchema.safeParse({ mac: 'not-a-mac', interface: 'br-lan' });
    expect(r.success).toBe(false);
  });
});

describe('dnsConfigFormSchema', () => {
  it('allows empty servers when custom DNS off', () => {
    const r = dnsConfigFormSchema.safeParse({
      use_custom_dns: false,
      server1: '',
      server2: '',
    });
    expect(r.success).toBe(true);
  });

  it('requires primary when custom DNS on', () => {
    const r = dnsConfigFormSchema.safeParse({
      use_custom_dns: true,
      server1: '',
      server2: '1.1.1.1',
    });
    expect(r.success).toBe(false);
  });
});

describe('ddnsFormSchema', () => {
  it('allows disabled with empty fields', () => {
    const r = ddnsFormSchema.safeParse({
      enabled: false,
      service: '',
      domain: '',
      username: '',
      password: '',
      lookup_host: '',
      update_url: '',
    });
    expect(r.success).toBe(true);
  });

  it('requires domain when enabled', () => {
    const r = ddnsFormSchema.safeParse({
      enabled: true,
      service: 'duckdns.org',
      domain: ' ',
      username: '',
      password: '',
      lookup_host: '',
      update_url: '',
    });
    expect(r.success).toBe(false);
  });

  it('requires update URL for custom provider', () => {
    const r = ddnsFormSchema.safeParse({
      enabled: true,
      service: 'custom',
      domain: 'x.example.com',
      username: '',
      password: '',
      lookup_host: '',
      update_url: '',
    });
    expect(r.success).toBe(false);
  });
});

describe('diagnosticsFormSchema', () => {
  it('accepts ping with target', () => {
    const r = diagnosticsFormSchema.safeParse({ type: 'ping', target: ' 8.8.8.8 ' });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.target).toBe('8.8.8.8');
  });

  it('rejects empty target', () => {
    const r = diagnosticsFormSchema.safeParse({ type: 'dns', target: '   ' });
    expect(r.success).toBe(false);
  });
});

describe('dnsEntryFormSchema', () => {
  it('accepts hostname and IPv4', () => {
    const r = dnsEntryFormSchema.safeParse({ name: 'router', ip: '192.168.1.1' });
    expect(r.success).toBe(true);
  });

  it('rejects non-IPv4-shaped string', () => {
    const r = dnsEntryFormSchema.safeParse({ name: 'x', ip: 'not-an-ip' });
    expect(r.success).toBe(false);
  });
});

describe('dhcpReservationFormSchema', () => {
  it('accepts name, MAC, IPv4', () => {
    const r = dhcpReservationFormSchema.safeParse({
      name: 'pc',
      mac: 'AA:BB:CC:DD:EE:FF',
      ip: '192.168.1.50',
    });
    expect(r.success).toBe(true);
  });

  it('rejects bad MAC', () => {
    const r = dhcpReservationFormSchema.safeParse({
      name: 'pc',
      mac: 'not-mac',
      ip: '192.168.1.50',
    });
    expect(r.success).toBe(false);
  });
});

describe('dhcpPoolFormSchema', () => {
  it('accepts valid pool', () => {
    const r = dhcpPoolFormSchema.safeParse({ start: 100, limit: 50, lease_time: '12h' });
    expect(r.success).toBe(true);
  });

  it('rejects start out of range', () => {
    const r = dhcpPoolFormSchema.safeParse({ start: 300, limit: 10, lease_time: '12h' });
    expect(r.success).toBe(false);
  });

  it('normalizeDhcpLeaseTime falls back for unknown', () => {
    expect(normalizeDhcpLeaseTime('weird')).toBe('12h');
    expect(normalizeDhcpLeaseTime('24h')).toBe('24h');
  });

  it('formatDhcpLeaseTimeHumanLabel maps options', () => {
    expect(formatDhcpLeaseTimeHumanLabel('1h')).toBe('1 hour');
    expect(formatDhcpLeaseTimeHumanLabel('7d')).toBe('7 days');
  });
});

describe('portForwardFormSchema', () => {
  it('accepts full rule', () => {
    const r = portForwardFormSchema.safeParse({
      name: 'web',
      protocol: 'tcp',
      src_dport: '443',
      dest_ip: '192.168.1.2',
      dest_port: '443',
    });
    expect(r.success).toBe(true);
  });

  it('rejects empty name', () => {
    const r = portForwardFormSchema.safeParse({
      name: ' ',
      protocol: 'tcp',
      src_dport: '80',
      dest_ip: '192.168.1.1',
      dest_port: '80',
    });
    expect(r.success).toBe(false);
  });
});

describe('dataBudgetFormSchema', () => {
  it('accepts limit and warn %', () => {
    const r = dataBudgetFormSchema.safeParse({ limit_gb: '100', warning_threshold_pct: '85' });
    expect(r.success).toBe(true);
  });

  it('rejects warn above 100', () => {
    const r = dataBudgetFormSchema.safeParse({ limit_gb: '10', warning_threshold_pct: '101' });
    expect(r.success).toBe(false);
  });
});
