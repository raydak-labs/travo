import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWsSubscribe } from '@/lib/ws-context';

/**
 * Refetches the given queries once the WebSocket comes back.
 *
 * Queries that stop polling while connected (`refetchInterval: false`) trust
 * the socket for liveness. Anything pushed while it was down is gone, so the
 * cache would otherwise hold pre-outage state until the window regains focus
 * and even then only if it went stale.
 */
export function useRefetchOnWsReconnect(queryKeys: ReadonlyArray<readonly unknown[]>): void {
  const queryClient = useQueryClient();
  const { connected } = useWsSubscribe();
  const wasDisconnectedRef = useRef(false);
  // Held in a ref so a caller passing an inline array does not re-run the effect.
  const keysRef = useRef(queryKeys);
  keysRef.current = queryKeys;

  useEffect(() => {
    if (!connected) {
      wasDisconnectedRef.current = true;
      return;
    }
    if (!wasDisconnectedRef.current) return;
    wasDisconnectedRef.current = false;
    for (const queryKey of keysRef.current) {
      void queryClient.invalidateQueries({ queryKey });
    }
  }, [connected, queryClient]);
}
