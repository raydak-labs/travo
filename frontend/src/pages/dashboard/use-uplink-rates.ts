import { useMemo } from 'react';
import { useWebSocket } from '@/hooks/use-websocket';
import { latestRate, type TransferRate } from '@/lib/traffic-series';
import { uplinkInterfaceName } from '@/lib/uplink';

export interface UplinkRates {
  /** Bytes per second received right now, or `null` before two samples exist. */
  readonly rx: number | null;
  /** Bytes per second sent right now, or `null` before two samples exist. */
  readonly tx: number | null;
  /** Newest interval for every interface the socket has sampled. */
  readonly rates: Readonly<Record<string, TransferRate>>;
}

/**
 * Live per-second rates for the uplink that carries the internet.
 *
 * Uses only the WebSocket tail on purpose: the dashboard answers "what is
 * happening now". Deriving a rate from the retained history would show a
 * minutes-old figure as if it were current.
 *
 * The per-interface map exists so the topology can annotate the source that
 * actually carries the traffic, not just the primary WAN name.
 */
export function useUplinkRates(wanName: string | null): UplinkRates {
  const { interfaceDataPoints } = useWebSocket();
  const rates = useMemo(() => {
    const next: Record<string, TransferRate> = {};
    for (const [name, points] of Object.entries(interfaceDataPoints)) {
      const rate = latestRate(points);
      if (rate) next[name] = rate;
    }
    return next;
  }, [interfaceDataPoints]);

  const interfaceName = uplinkInterfaceName(Object.keys(interfaceDataPoints), wanName);
  const rate = interfaceName ? rates[interfaceName] : undefined;
  return { rx: rate?.rx ?? null, tx: rate?.tx ?? null, rates };
}
