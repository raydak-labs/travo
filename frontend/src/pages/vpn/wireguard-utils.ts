export function formatWireguardHandshakeTime(epoch: number): string {
  if (epoch === 0) return 'Never';
  const now = Math.floor(Date.now() / 1000);
  const diff = now - epoch;
  if (diff < 60) return `${diff} seconds ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)} minutes ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} hours ago`;
  return `${Math.floor(diff / 86400)} days ago`;
}
