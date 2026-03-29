import { z } from 'zod';

/** Wake-on-LAN — MAC in common colon or hyphen form. */
export const wolFormSchema = z.object({
  mac: z
    .string()
    .trim()
    .regex(
      /^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/,
      'Enter a valid MAC address (e.g. AA:BB:CC:DD:EE:FF)',
    ),
  interface: z.string().trim(),
});

export type WolFormValues = z.infer<typeof wolFormSchema>;

/** LAN DNS — primary required when custom DNS is enabled. */
export const dnsConfigFormSchema = z
  .object({
    use_custom_dns: z.boolean(),
    server1: z.string(),
    server2: z.string(),
  })
  .superRefine((data, ctx) => {
    if (!data.use_custom_dns) return;
    if (!data.server1.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Primary DNS is required',
        path: ['server1'],
      });
    }
  });

export type DnsConfigFormValues = z.infer<typeof dnsConfigFormSchema>;

/** DDNS — when enabled, provider + domain required; custom provider needs update URL. */
export const ddnsFormSchema = z
  .object({
    enabled: z.boolean(),
    service: z.string(),
    domain: z.string(),
    username: z.string(),
    password: z.string(),
    lookup_host: z.string(),
    update_url: z.string(),
  })
  .superRefine((data, ctx) => {
    if (!data.enabled) return;
    if (!data.service.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Select a provider',
        path: ['service'],
      });
      return;
    }
    if (data.service === 'custom' && !data.update_url.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Update URL is required for custom provider',
        path: ['update_url'],
      });
    }
    if (!data.domain.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Domain is required',
        path: ['domain'],
      });
    }
  });

export type DdnsFormValues = z.infer<typeof ddnsFormSchema>;

const macAddressRegex = /^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/;
const ipv4LikeRegex = /^(\d{1,3}\.){3}\d{1,3}$/;

/** Network diagnostics (ping / traceroute / dns). */
export const diagnosticsFormSchema = z.object({
  type: z.enum(['ping', 'traceroute', 'dns']),
  target: z.string().trim().min(1, 'Enter a host or IP'),
});

export type DiagnosticsFormValues = z.infer<typeof diagnosticsFormSchema>;

/** Static local DNS entry (hostname → IPv4). */
export const dnsEntryFormSchema = z.object({
  name: z.string().trim().min(1, 'Hostname is required'),
  ip: z.string().trim().regex(ipv4LikeRegex, 'Enter a valid IPv4 address'),
});

export type DnsEntryFormValues = z.infer<typeof dnsEntryFormSchema>;

/** DHCP reservation row. */
export const dhcpReservationFormSchema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  mac: z
    .string()
    .trim()
    .regex(macAddressRegex, 'Enter a valid MAC address (e.g. AA:BB:CC:DD:EE:FF)'),
  ip: z.string().trim().regex(ipv4LikeRegex, 'Enter a valid IPv4 address'),
});

export type DhcpReservationFormValues = z.infer<typeof dhcpReservationFormSchema>;

/** Values accepted by the API / UCI for DHCP lease duration. */
export const DHCP_LEASE_TIME_OPTIONS = ['1h', '2h', '6h', '12h', '24h', '48h', '7d'] as const;

/** LAN DHCP pool (start offset, size, lease). */
export const dhcpPoolFormSchema = z.object({
  start: z.coerce
    .number()
    .int()
    .min(2, 'Must be between 2 and 254')
    .max(254, 'Must be between 2 and 254'),
  limit: z.coerce
    .number()
    .int()
    .min(1, 'Must be between 1 and 253')
    .max(253, 'Must be between 1 and 253'),
  lease_time: z.enum(DHCP_LEASE_TIME_OPTIONS),
});

export type DhcpPoolFormValues = z.infer<typeof dhcpPoolFormSchema>;

/** Map API lease string to a value accepted by `dhcpPoolFormSchema` (fallback `12h`). */
export function normalizeDhcpLeaseTime(v: string): DhcpPoolFormValues['lease_time'] {
  return DHCP_LEASE_TIME_OPTIONS.includes(v as DhcpPoolFormValues['lease_time'])
    ? (v as DhcpPoolFormValues['lease_time'])
    : '12h';
}

/** Human-readable label for a `DHCP_LEASE_TIME_OPTIONS` value (UI select). */
export function formatDhcpLeaseTimeHumanLabel(opt: DhcpPoolFormValues['lease_time']): string {
  switch (opt) {
    case '1h':
      return '1 hour';
    case '2h':
      return '2 hours';
    case '6h':
      return '6 hours';
    case '12h':
      return '12 hours';
    case '24h':
      return '24 hours';
    case '48h':
      return '48 hours';
    case '7d':
      return '7 days';
    default:
      return opt;
  }
}

/** Add firewall port-forward rule (DNAT). */
export const portForwardFormSchema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  protocol: z.enum(['tcp', 'udp', 'tcp udp']),
  src_dport: z.string().trim().min(1, 'External port is required'),
  dest_ip: z.string().trim().min(1, 'Internal IP is required'),
  dest_port: z.string().trim().min(1, 'Internal port is required'),
});

export type PortForwardFormValues = z.infer<typeof portForwardFormSchema>;

/** vnstat data budget: monthly cap (GB) + warning threshold (%). */
export const dataBudgetFormSchema = z.object({
  limit_gb: z
    .string()
    .trim()
    .min(1, 'Enter a monthly data limit in GB')
    .refine((s) => {
      const n = parseFloat(s);
      return !isNaN(n) && n > 0;
    }, 'Enter a positive number'),
  warning_threshold_pct: z
    .string()
    .trim()
    .min(1, 'Enter a warning threshold')
    .refine((s) => {
      const n = parseFloat(s);
      return !isNaN(n) && n >= 0 && n <= 100;
    }, 'Enter a percentage from 0 to 100'),
});

export type DataBudgetFormValues = z.infer<typeof dataBudgetFormSchema>;
