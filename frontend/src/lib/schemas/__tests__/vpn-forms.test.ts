import { describe, it, expect } from 'vitest';
import {
  wireguardProfileImportFormSchema,
  tailscaleAuthFormSchema,
  splitTunnelFormSchema,
} from '../vpn-forms';

describe('wireguardProfileImportFormSchema', () => {
  it('accepts name and config', () => {
    const r = wireguardProfileImportFormSchema.safeParse({
      name: 'Home',
      config: '[Interface]\nPrivateKey=x\n',
    });
    expect(r.success).toBe(true);
  });

  it('rejects empty config', () => {
    const r = wireguardProfileImportFormSchema.safeParse({ name: 'x', config: '   ' });
    expect(r.success).toBe(false);
  });
});

describe('tailscaleAuthFormSchema', () => {
  it('allows empty key', () => {
    const r = tailscaleAuthFormSchema.safeParse({ auth_key: '' });
    expect(r.success).toBe(true);
  });
});

describe('splitTunnelFormSchema', () => {
  it('accepts all mode', () => {
    const r = splitTunnelFormSchema.safeParse({ mode: 'all', routes_text: '' });
    expect(r.success).toBe(true);
  });

  it('accepts custom with routes text', () => {
    const r = splitTunnelFormSchema.safeParse({
      mode: 'custom',
      routes_text: '10.0.0.0/8, 192.168.0.0/16',
    });
    expect(r.success).toBe(true);
  });
});
