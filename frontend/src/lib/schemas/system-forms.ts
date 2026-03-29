import { z } from 'zod';

/** AdGuard Home password change (no current password required — admin-level). */
export const changeAdGuardPasswordSchema = z
  .object({
    new_password: z.string().min(6, 'Password must be at least 6 characters'),
    confirm_password: z.string().min(1, 'Confirm your new password'),
  })
  .refine((data) => data.new_password === data.confirm_password, {
    message: 'Passwords do not match',
    path: ['confirm_password'],
  });

export type ChangeAdGuardPasswordFormValues = z.infer<typeof changeAdGuardPasswordSchema>;

/** System page — change admin password (API allows new password ≥ 6 chars). */
export const changeAdminPasswordSchema = z
  .object({
    current_password: z.string().min(1, 'Current password is required'),
    new_password: z.string().min(6, 'Password must be at least 6 characters'),
    confirm_password: z.string().min(1, 'Confirm your new password'),
  })
  .refine((data) => data.new_password === data.confirm_password, {
    message: 'Passwords do not match',
    path: ['confirm_password'],
  });

export type ChangeAdminPasswordFormValues = z.infer<typeof changeAdminPasswordSchema>;

export const hostnameFormSchema = z.object({
  hostname: z.string().trim().min(1, 'Hostname is required').max(253, 'Hostname is too long'),
});

export type HostnameFormValues = z.infer<typeof hostnameFormSchema>;

/** Alert thresholds (50–99%, matches UI sliders). */
export const alertThresholdsFormSchema = z.object({
  storage_percent: z.number().int().min(50).max(99),
  cpu_percent: z.number().int().min(50).max(99),
  memory_percent: z.number().int().min(50).max(99),
});

export type AlertThresholdsFormValues = z.infer<typeof alertThresholdsFormSchema>;

/** Paste box for `POST /system/ssh-keys`. */
export const sshPublicKeyFormSchema = z.object({
  key: z
    .string()
    .trim()
    .min(1, 'Paste a public key')
    .refine((s) => s.startsWith('ssh-') || s.startsWith('ecdsa-'), {
      message: 'Key must start with ssh- or ecdsa-',
    }),
});

export type SshPublicKeyFormValues = z.infer<typeof sshPublicKeyFormSchema>;

const ntpServerRowSchema = z.object({
  value: z.string(),
});

/** NTP form — `servers` rows use `value`; API receives trimmed non-empty strings. */
export const ntpConfigFormSchema = z
  .object({
    enabled: z.boolean(),
    servers: z.array(ntpServerRowSchema),
  })
  .refine(
    (data) => {
      if (!data.enabled) return true;
      return data.servers.some((s) => s.value.trim().length > 0);
    },
    {
      message: 'Add at least one NTP server when NTP is enabled',
      path: ['servers'],
    },
  );

export type NtpConfigFormValues = z.infer<typeof ntpConfigFormSchema>;

/** Single-row “add NTP server” before appending to the field array. */
export const ntpServerDraftSchema = z.object({
  server: z.string().trim().min(1, 'Enter an NTP server').max(256),
});

export type NtpServerDraftFormValues = z.infer<typeof ntpServerDraftSchema>;

/** HTML time input (HH:MM). */
const timeHHMMSchema = z.string().regex(/^([01]\d|2[0-3]):[0-5]\d$/, 'Use a valid time (HH:MM)');

/** LED schedule — matches `LEDSchedule` API shape. */
export const ledScheduleFormSchema = z.object({
  enabled: z.boolean(),
  on_time: timeHHMMSchema,
  off_time: timeHHMMSchema,
});

export type LedScheduleFormValues = z.infer<typeof ledScheduleFormSchema>;

export const buttonActionSchema = z.enum([
  'none',
  'vpn_toggle',
  'wifi_toggle',
  'led_toggle',
  'reboot',
]);

export const hardwareButtonsFormSchema = z.object({
  buttons: z.array(
    z.object({
      name: z.string().min(1),
      action: buttonActionSchema,
    }),
  ),
});

export type HardwareButtonsFormValues = z.infer<typeof hardwareButtonsFormSchema>;

/** Firmware flash — file set via `setValue` after picker. */
export const firmwareUpgradeFormSchema = z
  .object({
    keep_settings: z.boolean(),
    firmware: z.custom<File | null>((v) => v === null || v instanceof File),
  })
  .refine((d) => d.firmware instanceof File, {
    message: 'Select a firmware image (.bin)',
    path: ['firmware'],
  });

export type FirmwareUpgradeFormValues = z.infer<typeof firmwareUpgradeFormSchema>;
