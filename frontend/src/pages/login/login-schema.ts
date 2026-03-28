import { z } from 'zod';

/** Login form — validated client-side before calling the auth API. */
export const loginFormSchema = z.object({
  password: z.string().min(1, 'Password is required'),
  rememberMe: z.boolean(),
});

export type LoginFormValues = z.infer<typeof loginFormSchema>;
