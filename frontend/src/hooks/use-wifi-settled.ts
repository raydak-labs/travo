/**
 * The shared settle step of every wireless mutation.
 *
 * A WiFi change perturbs every `wifi` query plus a few router-wide ones, so
 * this lives on its own: the connection, AP and network modules all call it.
 */
import type { QueryClient } from '@tanstack/react-query';
import { refreshRouterState } from '@/lib/router-state-refresh';

/**
 * A WiFi change perturbs every `wifi` query plus a few router-wide ones.
 *
 * The broad invalidate covers all `wifi` keys (including ones that need no
 * follow-up, like the radio scan); `refreshRouterState` then only repeats the
 * delayed passes on `refreshKeys`. Calling both in full fetched the listed keys
 * four times per mutation.
 */
export function wifiMutationSettled(queryClient: QueryClient, refreshKeys: Array<readonly unknown[]>) {
  void queryClient.invalidateQueries({ queryKey: ['wifi'] });
  for (const queryKey of refreshKeys) {
    void queryClient.invalidateQueries({ queryKey });
  }
  void refreshRouterState(queryClient, refreshKeys, { skipImmediate: true });
}
