import { z } from 'zod';

/** Logs page client-side filters (all strings; booleans for UI mode). */
export const logsFilterFormSchema = z.object({
  lineFilter: z.string(),
  serviceFilter: z.string(),
  levelFilter: z.string(),
  customService: z.string(),
  showCustomInput: z.boolean(),
});

export type LogsFilterFormValues = z.infer<typeof logsFilterFormSchema>;
