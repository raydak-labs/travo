import { describe, it, expect } from 'vitest';
import {
  changeAdminPasswordSchema,
  hostnameFormSchema,
  alertThresholdsFormSchema,
  sshPublicKeyFormSchema,
  ntpConfigFormSchema,
  ntpServerDraftSchema,
  ledScheduleFormSchema,
  hardwareButtonsFormSchema,
  firmwareUpgradeFormSchema,
} from '../system-forms';

describe('changeAdminPasswordSchema', () => {
  it('accepts matching passwords with new password length ≥ 6', () => {
    const r = changeAdminPasswordSchema.safeParse({
      current_password: 'old',
      new_password: 'secret',
      confirm_password: 'secret',
    });
    expect(r.success).toBe(true);
  });

  it('rejects short new password', () => {
    const r = changeAdminPasswordSchema.safeParse({
      current_password: 'old',
      new_password: 'short',
      confirm_password: 'short',
    });
    expect(r.success).toBe(false);
  });

  it('rejects mismatched confirm', () => {
    const r = changeAdminPasswordSchema.safeParse({
      current_password: 'old',
      new_password: 'secret1',
      confirm_password: 'secret2',
    });
    expect(r.success).toBe(false);
  });
});

describe('hostnameFormSchema', () => {
  it('trims and requires non-empty hostname', () => {
    const r = hostnameFormSchema.safeParse({ hostname: '  router  ' });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.hostname).toBe('router');
  });

  it('rejects empty hostname', () => {
    const r = hostnameFormSchema.safeParse({ hostname: '   ' });
    expect(r.success).toBe(false);
  });
});

describe('alertThresholdsFormSchema', () => {
  it('accepts values in 50–99', () => {
    const r = alertThresholdsFormSchema.safeParse({
      storage_percent: 75,
      cpu_percent: 80,
      memory_percent: 90,
    });
    expect(r.success).toBe(true);
  });

  it('rejects out-of-range values', () => {
    const r = alertThresholdsFormSchema.safeParse({
      storage_percent: 40,
      cpu_percent: 90,
      memory_percent: 90,
    });
    expect(r.success).toBe(false);
  });
});

describe('sshPublicKeyFormSchema', () => {
  it('accepts ssh-ed25519 line', () => {
    const r = sshPublicKeyFormSchema.safeParse({
      key: 'ssh-ed25519 AAAAcomment user@host',
    });
    expect(r.success).toBe(true);
  });

  it('rejects non-SSH lines', () => {
    const r = sshPublicKeyFormSchema.safeParse({ key: 'hello world' });
    expect(r.success).toBe(false);
  });
});

describe('ntpConfigFormSchema', () => {
  it('allows disabled with empty servers', () => {
    const r = ntpConfigFormSchema.safeParse({ enabled: false, servers: [] });
    expect(r.success).toBe(true);
  });

  it('requires at least one server when enabled', () => {
    const r = ntpConfigFormSchema.safeParse({ enabled: true, servers: [] });
    expect(r.success).toBe(false);
  });

  it('accepts enabled with one server', () => {
    const r = ntpConfigFormSchema.safeParse({
      enabled: true,
      servers: [{ value: 'pool.ntp.org' }],
    });
    expect(r.success).toBe(true);
  });
});

describe('ntpServerDraftSchema', () => {
  it('accepts non-empty trimmed server', () => {
    const r = ntpServerDraftSchema.safeParse({ server: '  pool.ntp.org  ' });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.server).toBe('pool.ntp.org');
  });

  it('rejects empty server', () => {
    const r = ntpServerDraftSchema.safeParse({ server: '   ' });
    expect(r.success).toBe(false);
  });
});

describe('ledScheduleFormSchema', () => {
  it('accepts valid times', () => {
    const r = ledScheduleFormSchema.safeParse({
      enabled: true,
      on_time: '07:00',
      off_time: '22:30',
    });
    expect(r.success).toBe(true);
  });

  it('rejects invalid time', () => {
    const r = ledScheduleFormSchema.safeParse({
      enabled: true,
      on_time: '25:00',
      off_time: '22:00',
    });
    expect(r.success).toBe(false);
  });
});

describe('hardwareButtonsFormSchema', () => {
  it('accepts button rows', () => {
    const r = hardwareButtonsFormSchema.safeParse({
      buttons: [
        { name: 'reset', action: 'reboot' as const },
        { name: 'wps', action: 'none' as const },
      ],
    });
    expect(r.success).toBe(true);
  });
});

describe('firmwareUpgradeFormSchema', () => {
  it('requires a File when refining', () => {
    const r = firmwareUpgradeFormSchema.safeParse({
      keep_settings: true,
      firmware: null,
    });
    expect(r.success).toBe(false);
  });

  it('accepts a File', () => {
    const file = new File([new Uint8Array([1])], 'sysupgrade.bin');
    const r = firmwareUpgradeFormSchema.safeParse({
      keep_settings: false,
      firmware: file,
    });
    expect(r.success).toBe(true);
  });
});
