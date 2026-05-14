/**
 * Module-level singleton to prevent toast re-firing on navigation.
 * Portal URL keys are stored here so notifications survive page transitions.
 */
const notifiedPortals = new Set<string>();

export function notifyPortalOnce(portalUrl: string, onNotify: () => void): void {
  if (notifiedPortals.has(portalUrl)) return;
  notifiedPortals.add(portalUrl);
  onNotify();
}

export function clearPortalNotification(portalUrl: string): void {
  notifiedPortals.delete(portalUrl);
}

export function clearAllPortalNotifications(): void {
  notifiedPortals.clear();
}
