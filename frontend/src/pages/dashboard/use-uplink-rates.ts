import { useWebSocket } from '@/hooks/use-websocket';
import { latestRate } from '@/lib/traffic-series';
import { uplinkInterfaceName } from '@/lib/uplink';

export interface UplinkRates {
  /** Bytes per second received right now, or `null` before two samples exist. */
  readonly rx: number | null;
  /** Bytes per second sent right now, or `null` before two samples exist. */
  readonly tx: number | null;
}

/**
 * Live per-second rates for the uplink that carries the internet.
 *
 * Uses only the WebSocket tail on purpose: the dashboard answers "what is
 * happening now". Deriving a rate from the retained history would show a
 * minutes-old figure as if it were current.
 */
export function useUplinkRates(wanName: string | null): UplinkRates {
  const { interfaceDataPoints } = useWebSocket();
  const interfaceName = uplinkInterfaceName(Object.keys(interfaceDataPoints), wanName);
  const rate = latestRate(interfaceName ? (interfaceDataPoints[interfaceName] ?? []) : []);
  return { rx: rate?.rx ?? null, tx: rate?.tx ?? null };
}
