import { z } from 'zod';

/** Import WireGuard profile from pasted config or uploaded .conf. */
export const wireguardProfileImportFormSchema = z.object({
  name: z.string().trim().min(1, 'Profile name is required'),
  config: z.string().trim().min(1, 'Add a WireGuard config (paste or upload a .conf file)'),
});

export type WireguardProfileImportFormValues = z.infer<typeof wireguardProfileImportFormSchema>;

/** Tailscale login — optional pre-auth key (empty = interactive / URL flow). */
export const tailscaleAuthFormSchema = z.object({
  auth_key: z.string().max(2048),
});

export type TailscaleAuthFormValues = z.infer<typeof tailscaleAuthFormSchema>;

/** VPN split tunneling: all traffic vs custom CIDR list. */
export const splitTunnelFormSchema = z.object({
  mode: z.enum(['all', 'custom']),
  routes_text: z.string(),
});

export type SplitTunnelFormValues = z.infer<typeof splitTunnelFormSchema>;
