// Dashboard page: topology diagram, uplink cards, quick status and actions.

import { Cable, Wifi, Smartphone, Shield, Monitor } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { TrafficRate } from '@/components/traffic-rate';
import { TimezoneAlert } from '@/components/timezone-alert';
import { useTopologyData } from '@/hooks/use-topology-data';
import { UPLINK_INACTIVE, UPLINK_LABELS } from '@/lib/uplink';
import { formatUptime } from '@/lib/utils';
import { QuickActions } from '@/pages/dashboard/quick-actions';
import { TopologyDiagram } from '@/pages/dashboard/topology-diagram';
import { SourceCard, DetailRow } from '@/pages/dashboard/source-card';
import { useUplinkRates } from '@/pages/dashboard/use-uplink-rates';

// ─── Main page ────────────────────────────────────────────────────────────────

/** Human labels for the WAN connection protocol reported by the WAN config. */
/** Labels for the common UCI protos; any other renders as its raw name. */
const WAN_PROTOCOL_LABEL: Partial<Record<string, string>> = {
  dhcp: 'DHCP',
  static: 'Static',
  pppoe: 'PPPoE',
  usb_tethering: 'USB tethering',
  none: 'None',
};

export function DashboardPage() {
  const {
    sources,
    clients,
    features,
    router,
    loading,
    ethernetUp,
    repeaterUp,
    tetherUp,
    wan,
    wanMedium,
    wanProtocol,
    wifiConn,
    usbTether,
    sysInfo,
    vpn: vpnStatus,
    internetUp,
    allClients,
  } = useTopologyData();
  const { rx, tx, rates } = useUplinkRates(wan?.name ?? null);
  // Rates belong to the interface, not the source label. Attach them here so
  // the graph never has to know how the uplink was resolved.
  const sourcesWithRates = sources.map((source) => ({
    ...source,
    rate: source.connected && source.interfaceName ? (rates[source.interfaceName] ?? null) : null,
  }));

  // The interface discriminator is the medium, not an address: prefer the
  // tethering service's own report and fall back to the USB WAN interface.
  const usbDisplayIp =
    usbTether?.ip_address ||
    (wan?.is_up === true && wanMedium === 'usb' ? wan.ip_address : '') ||
    '';

  return (
    <div className="space-y-6">
      <TimezoneAlert />

      <TopologyDiagram
        sources={sourcesWithRates}
        clients={clients}
        hostname={router.hostname}
        model={router.model}
        features={features}
        loading={loading}
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <SourceCard title={UPLINK_LABELS.wan} icon={Cable} connected={ethernetUp}>
          {ethernetUp && wan ? (
            <>
              <DetailRow label="Protocol">
                {/* The WAN protocol lives in the WAN config, not in the
                    interface discriminator (which only says "wired").

                    The backend passes UCI's `proto` through unvalidated, and
                    OpenWrt accepts more than the typed union (dhcpv6, 6rd,
                    6to4, wwan, qmi, ncm, directip). Fall back to the raw value
                    so an unlisted proto renders as text rather than as an
                    empty cell. */}
                <span className="uppercase">
                  {wanProtocol ? (WAN_PROTOCOL_LABEL[wanProtocol] ?? wanProtocol) : '—'}
                </span>
              </DetailRow>
              {wan.ip_address && (
                <DetailRow label="IP" mono>
                  {wan.ip_address}
                </DetailRow>
              )}
              {wan.gateway && (
                <DetailRow label="Gateway" mono>
                  {wan.gateway}
                </DetailRow>
              )}
            </>
          ) : (
            <p className="text-gray-500 dark:text-gray-500">No Ethernet WAN connection detected.</p>
          )}
        </SourceCard>

        <SourceCard title={UPLINK_LABELS.wwan} icon={Wifi} connected={repeaterUp}>
          {repeaterUp && wifiConn ? (
            <>
              <DetailRow label="SSID" mono>
                {wifiConn.ssid}
              </DetailRow>
              <DetailRow label="Band">
                <span className="uppercase">{wifiConn.band}</span>
              </DetailRow>
              <DetailRow label="Signal">{wifiConn.signal_percent}%</DetailRow>
            </>
          ) : (
            <p className="text-gray-500 dark:text-gray-500">Repeater (STA) is disabled.</p>
          )}
        </SourceCard>

        <SourceCard title={UPLINK_LABELS.usbtether} icon={Smartphone} connected={tetherUp}>
          {tetherUp ? (
            <>
              {usbTether?.detected && (
                <DetailRow label="Device">
                  <span className="capitalize">{usbTether.device_type || 'Unknown'}</span>
                </DetailRow>
              )}
              {usbDisplayIp ? (
                <DetailRow label="IP" mono>
                  {usbDisplayIp}
                </DetailRow>
              ) : null}
              {tetherUp && !usbTether?.detected && !usbDisplayIp ? (
                <p className="text-gray-500 dark:text-gray-500">USB uplink is active.</p>
              ) : null}
            </>
          ) : (
            <p className="text-gray-500 dark:text-gray-500">
              {UPLINK_INACTIVE.usbtether} Plug in your smartphone or USB modem to start.
            </p>
          )}
        </SourceCard>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Quick status</CardTitle>
          <Monitor className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          ) : (
            <>
              <div className="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
                <div>
                  {/* `internet_reachable` is a WAN-carrier check, not a
                      reachability probe: a DHCP lease on a dead or
                      captive-portaled uplink still reads true. Labelling it
                      "Internet" is the one word a non-expert trusts most. */}
                  <span className="text-gray-500 dark:text-gray-400">Uplink</span>
                  <p
                    className={`mt-0.5 font-medium ${
                      internetUp
                        ? 'text-[var(--status-ok-text)]'
                        : 'text-[var(--status-danger-text)]'
                    }`}
                  >
                    {internetUp ? 'Connected' : 'Disconnected'}
                  </p>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">VPN</span>
                  {/* Distinguishes configured-but-not-connected from off, so this
                      no longer reads "Off" directly above a "Disable VPN"
                      button. */}
                  <div className="mt-0.5 flex items-center gap-1.5">
                    <Shield
                      aria-hidden="true"
                      className={
                        vpnStatus.healthy
                          ? 'h-3.5 w-3.5 text-[var(--status-ok-text)]'
                          : vpnStatus.active
                            ? 'h-3.5 w-3.5 text-[var(--status-warn-text)]'
                            : 'h-3.5 w-3.5 text-gray-500 dark:text-gray-400'
                      }
                    />
                    <span
                      className={
                        vpnStatus.healthy
                          ? 'font-medium text-[var(--status-ok-text)]'
                          : vpnStatus.active
                            ? 'font-medium text-[var(--status-warn-text)]'
                            : 'font-medium text-gray-500 dark:text-gray-400'
                      }
                    >
                      {vpnStatus.label}
                    </span>
                  </div>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">Traffic</span>
                  <div className="mt-0.5">
                    <TrafficRate rxBytesPerSec={rx} txBytesPerSec={tx} />
                  </div>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">Devices on your network</span>
                  <p className="mt-0.5 font-medium">{allClients.length}</p>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">Uptime</span>
                  <p className="mt-0.5 font-medium">
                    {sysInfo ? formatUptime(sysInfo.uptime_seconds) : '—'}
                  </p>
                </div>
              </div>
              <p className="mt-4 text-sm text-gray-500 dark:text-gray-400">
                <Link to="/system" className="underline-offset-4 hover:underline">
                  Device details and settings
                </Link>{' '}
                — hostname, firmware, CPU, and storage.
              </p>
            </>
          )}
        </CardContent>
      </Card>

      <QuickActions />
    </div>
  );
}
