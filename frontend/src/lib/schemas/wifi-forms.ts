import { z } from 'zod';

/** Scan-result connect dialog — password rules depend on whether the network is encrypted. */
export function createWifiConnectFormSchema(needsPassword: boolean) {
  return z.object({
    password: needsPassword
      ? z.string().min(8, 'Password must be at least 8 characters')
      : z.string(),
    selectedBand: z.string(),
  });
}

export type WifiConnectFormValues = z.infer<ReturnType<typeof createWifiConnectFormSchema>>;

const hiddenEncryptionEnum = z.enum(['psk2', 'sae', 'psk', 'none']);

/** Hidden-network dialog (manual SSID + encryption + optional PSK). */
export const wifiHiddenNetworkFormSchema = z
  .object({
    ssid: z.string().trim().min(1, 'SSID is required'),
    encryption: hiddenEncryptionEnum,
    password: z.string(),
  })
  .superRefine((data, ctx) => {
    if (data.encryption === 'none') return;
    if (data.password.length > 0 && data.password.length < 8) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Password must be at least 8 characters',
        path: ['password'],
      });
      return;
    }
    if (data.password.length === 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Password is required for encrypted networks',
        path: ['password'],
      });
    }
  });

export type WifiHiddenNetworkFormValues = z.infer<typeof wifiHiddenNetworkFormSchema>;

const macColonHex = /^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/;

/** Per-network MAC policy row (SSID required; MAC optional = device default). */
export const macPolicyAddFormSchema = z
  .object({
    ssid: z.string().trim().min(1, 'SSID is required'),
    mac: z.string().trim(),
  })
  .superRefine((data, ctx) => {
    if (!data.mac) return;
    if (!macColonHex.test(data.mac)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Invalid MAC address format (e.g. aa:bb:cc:dd:ee:ff)',
        path: ['mac'],
      });
    }
  });

export type MacPolicyAddFormValues = z.infer<typeof macPolicyAddFormSchema>;

const timeHHMM = z
  .string()
  .regex(/^([01]\d|2[0-3]):[0-5]\d$/, 'Use a valid time (HH:MM)');

/** WiFi on/off schedule (cron-backed). */
export const wifiScheduleFormSchema = z.object({
  enabled: z.boolean(),
  on_time: timeHHMM,
  off_time: timeHHMM,
});

export type WifiScheduleFormValues = z.infer<typeof wifiScheduleFormSchema>;

/** WPA options for guest SSID and per-radio AP config. */
export const wifiApEncryptionEnum = z.enum(['none', 'psk2', 'sae', 'psk-mixed']);

/** Isolated guest SSID (192.168.2.x); validation when `enabled` is true. */
export const guestWifiFormSchema = z
  .object({
    enabled: z.boolean(),
    ssid: z.string(),
    encryption: wifiApEncryptionEnum,
    key: z.string(),
  })
  .superRefine((data, ctx) => {
    if (!data.enabled) return;
    if (!data.ssid.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'SSID is required',
        path: ['ssid'],
      });
    }
    if (data.encryption === 'none') return;
    if (!data.key || data.key.length < 8) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Password must be at least 8 characters',
        path: ['key'],
      });
    }
  });

export type GuestWifiFormValues = z.infer<typeof guestWifiFormSchema>;

/** STA MAC clone — empty clears override; otherwise colon-separated hex. */
export const macCloneFormSchema = z.object({
  custom_mac: z
    .string()
    .trim()
    .refine(
      (s) => s === '' || /^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/.test(s),
      'Enter a valid MAC address (e.g. AA:BB:CC:DD:EE:FF)',
    ),
});

export type MacCloneFormValues = z.infer<typeof macCloneFormSchema>;

/** Access point radio — same fields/validation as guest WiFi. */
export const apRadioFormSchema = guestWifiFormSchema;
export type APRadioFormValues = GuestWifiFormValues;
