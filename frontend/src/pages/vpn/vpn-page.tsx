import { WireguardSection } from './wireguard-section';
import { TailscaleSection } from './tailscale-section';

export function VpnPage() {
  return (
    <div className="space-y-6">
      {/* WireGuard Section */}
      <WireguardSection />

      {/* Tailscale Section */}
      <TailscaleSection />
    </div>
  );
}
