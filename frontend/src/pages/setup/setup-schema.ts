import { z } from 'zod';

/** Step 2 — change admin password (current + new + confirm). */
export const setupPasswordFormSchema = z
  .object({
    current_password: z.string().min(1, 'Current password is required'),
    new_password: z.string().min(8, 'Password must be at least 8 characters'),
    confirm_password: z.string().min(1, 'Confirm your new password'),
  })
  .refine((data) => data.new_password === data.confirm_password, {
    message: 'Passwords do not match',
    path: ['confirm_password'],
  });

export type SetupPasswordFormValues = z.infer<typeof setupPasswordFormSchema>;

/** Step 4 — access point SSID + WPA key. */
export const setupApFormSchema = z.object({
  ssid: z.string().min(1, 'Network name is required'),
  key: z.string().min(8, 'Password must be at least 8 characters'),
});

export type SetupApFormValues = z.infer<typeof setupApFormSchema>;

/** Step 3 — upstream WiFi: SSID from scan + optional PSK when encrypted. */
export const setupWifiFormSchema = z
  .object({
    selectedSsid: z.string(),
    encryption: z.string().optional(),
    wifiPassword: z.string(),
  })
  .superRefine((data, ctx) => {
    if (!data.selectedSsid.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Select a network',
        path: ['selectedSsid'],
      });
    }
    const enc = data.encryption ?? 'none';
    if (enc !== 'none' && data.wifiPassword.length === 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Enter the network password',
        path: ['wifiPassword'],
      });
    }
  });

export type SetupWifiFormValues = z.infer<typeof setupWifiFormSchema>;
