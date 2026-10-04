import { useWsSubscribe } from '@/lib/ws-context';

/**
 * Whether the router is currently reachable.
 *
 * `navigator.onLine` answers a different question — "does this device have
 * *any* network". A phone that drops the router's WiFi but keeps cellular has
 * `navigator.onLine === true` while every `/api/v1` call fails, and a laptop on
 * a dead WAN reports offline while the router is perfectly reachable. The
 * WebSocket connection is the router-accurate signal, and it is already open.
 */
export function useRouterReachable(): boolean {
  return useWsSubscribe().connected;
}
