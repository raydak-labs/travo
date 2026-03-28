import { z } from 'zod';

/** Edit a DHCP client display name (empty clears custom alias). */
export const clientAliasFormSchema = z.object({
  alias: z.string().max(128, 'Alias is too long'),
});

export type ClientAliasFormValues = z.infer<typeof clientAliasFormSchema>;
