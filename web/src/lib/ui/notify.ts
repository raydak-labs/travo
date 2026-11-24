import { toast } from 'svelte-sonner';

/**
 * Results of an action that is not a guarded transaction (saved, deleted, failed).
 * `progress` + `done` with the same id turn one toast from a spinner into its result.
 * Errors stay until dismissed; the rest fade after five seconds.
 */
export const notify = {
  success: (text: string, id?: string) => toast.success(text, { id, duration: 5000 }),
  info: (text: string, id?: string) => toast.info(text, { id, duration: 5000 }),
  /** `detail` adds the fix under a title that names the failure. */
  error: (text: string, id?: string, detail?: string) =>
    toast.error(text, {
      id,
      description: detail,
      duration: Number.POSITIVE_INFINITY,
      closeButton: true,
    }),
  progress: (text: string, id: string) =>
    toast.loading(text, { id, duration: Number.POSITIVE_INFINITY, dismissible: false }),
  dismiss: (id: string) => toast.dismiss(id),
};
