import type { QueryClient } from '@tanstack/react-query';

function sleep(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms));
}

async function refetchAll(queryClient: QueryClient, queryKeys: Array<readonly unknown[]>) {
  await Promise.all(
    queryKeys.map((queryKey) =>
      queryClient.refetchQueries({ queryKey, type: 'active' }),
    ),
  );
}

/**
 * Some router operations (wifi connect/apply, vpn toggles) can succeed before
 * dependent status endpoints converge (netifd/dnsmasq route updates).
 *
 * We do an immediate refetch plus a short bounded follow-up refetch to avoid
 * requiring a manual browser reload.
 */
export async function refreshRouterState(
  queryClient: QueryClient,
  queryKeys: Array<readonly unknown[]>,
  opts?: { followUps?: number; followUpDelayMs?: number },
) {
  const followUps = opts?.followUps ?? 2;
  const followUpDelayMs = opts?.followUpDelayMs ?? 2000;

  await refetchAll(queryClient, queryKeys);

  for (let i = 0; i < followUps; i++) {
    await sleep(followUpDelayMs);
    await refetchAll(queryClient, queryKeys);
  }
}

