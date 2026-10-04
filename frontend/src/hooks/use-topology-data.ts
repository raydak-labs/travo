import { Cable, Wifi, Smartphone, Signal } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type { NetworkStatus, WanType } from '@shared/index';
import { networkMedium, type NetworkMedium } from '@shared/index';
import { useNetworkStatus, useIPv6Status, useWanConfig } from './use-network';
import { useWifiConnection } from './use-wifi';
import { useVpnStatus } from './use-vpn';
import { useSystemInfo } from './use-system';
import { useUSBTetherStatus } from './use-usb-tether';
import { useRefetchOnWsReconnect } from './use-refetch-on-ws-reconnect';
import { useWsSubscribe } from '@/lib/ws-context';

export interface SourceDef {
  label: string;
  icon: LucideIcon;
  connected: boolean;
  detail?: string;
}

export interface TopologyData {
  // Derived display state for TopologyDiagram
  sources: SourceDef[];
  clients: { label: string; icon: LucideIcon; count: number }[];
  features: { label: string; active: boolean }[];
  router: { hostname: string; model: string };
  loading: boolean;
  // Named booleans — no index-based access needed in consumers
  ethernetUp: boolean;
  repeaterUp: boolean;
  tetherUp: boolean;
  // Raw data for dashboard source / detail cards
  wan: NetworkStatus['wan'];
  /** Medium the WAN uplink runs over (ethernet / wifi / usb / …). */
  wanMedium: NetworkMedium;
  /** WAN connection protocol (dhcp / static / pppoe), from the WAN config. */
  wanProtocol: WanType | null;
  wifiConn: ReturnType<typeof useWifiConnection>['data'];
  usbTether: ReturnType<typeof useUSBTetherStatus>['data'];
  sysInfo: ReturnType<typeof useSystemInfo>['data'];
  vpnActive: boolean;
  ipv6Enabled: boolean;
  internetUp: boolean;
  allClients: NonNullable<NetworkStatus['clients']>;
}

/**
 * HTTP polling cadence for the topology query.
 *
 * While the WebSocket is connected its `network_status` pushes keep the cache
 * fresh, so polling is pure overhead; with the socket down (or before the
 * first login connects it) polling is what heals a frozen dashboard.
 * Exported for tests.
 */
export function topologyRefetchInterval(wsConnected: boolean): number | false {
  return wsConnected ? false : 15_000;
}

export function useTopologyData(): TopologyData {
  const { connected } = useWsSubscribe();
  // Anything pushed while the socket was down is gone for good; without this
  // the dashboard holds pre-outage WAN/client state indefinitely.
  useRefetchOnWsReconnect([['network', 'status']]);
  // A finite staleTime (instead of `Infinity`) plus focus/reconnect refetching
  // is what makes the dashboard heal when the WebSocket never delivered a
  // push: `staleTime: Infinity` also disabled refetchOnWindowFocus and
  // refetchOnReconnect, so a single failed fetch froze the page forever.
  const { data: network, isLoading: networkLoading } = useNetworkStatus({
    staleTime: 5_000,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
    refetchInterval: topologyRefetchInterval(connected),
  });
  const { data: wifiConn, isLoading: wifiLoading } = useWifiConnection();
  const { data: vpnStatus } = useVpnStatus();
  const { data: sysInfo, isLoading: sysLoading } = useSystemInfo();
  const { data: ipv6Status } = useIPv6Status();
  const { data: usbTether } = useUSBTetherStatus();
  const { data: wanConfig } = useWanConfig();

  // Connection type derivation — the interface discriminator says which
  // medium the uplink actually runs over (see NetworkInterfaceType).
  const wan = network?.wan ?? null;
  const wanMedium = networkMedium(wan);
  const ethernetUp = wan?.is_up === true && wanMedium === 'ethernet';
  const repeaterUp =
    (wan?.is_up === true && wanMedium === 'wifi') ||
    (wifiConn?.connected === true && wifiConn.mode === 'client');
  const tetherUp = (wan?.is_up === true && wanMedium === 'usb') || usbTether?.is_up === true;

  const vpnActive = vpnStatus?.some((v) => v.connected) ?? false;
  const ipv6Enabled = ipv6Status?.enabled ?? false;
  const internetUp = network?.internet_reachable ?? false;

  const allClients = network?.clients ?? [];
  const wlanClients = allClients.filter(
    (c) =>
      c.interface_name.startsWith('wlan') ||
      c.interface_name.startsWith('ath') ||
      c.interface_name.includes('wifi') ||
      c.interface_name.includes('-ap'),
  ).length;
  const lanClients = allClients.length - wlanClients;

  return {
    sources: [
      {
        label: 'Ethernet',
        icon: Cable,
        connected: ethernetUp,
        detail: ethernetUp ? (wan?.ip_address ?? undefined) : undefined,
      },
      {
        label: 'Repeater (WiFi)',
        icon: Wifi,
        connected: repeaterUp,
        detail: repeaterUp ? (wifiConn?.ssid ?? wan?.ip_address ?? undefined) : 'Disabled',
      },
      {
        label: 'USB Tethering',
        icon: Smartphone,
        connected: tetherUp,
        detail: tetherUp ? usbTether?.device_type || wan?.ip_address || 'Connected' : 'No device',
      },
      {
        label: 'Cellular',
        icon: Signal,
        connected: false,
        detail: 'No modem',
      },
    ],
    clients: [
      { label: 'WLAN Clients', icon: Wifi, count: wlanClients },
      { label: 'LAN Clients', icon: Cable, count: lanClients },
    ],
    features: [
      { label: 'IPv6', active: ipv6Enabled },
      { label: 'VPN', active: vpnActive },
      { label: 'Internet', active: internetUp },
    ],
    router: {
      hostname: sysInfo?.hostname ?? '',
      model: sysInfo?.model ?? '',
    },
    loading: networkLoading || wifiLoading || sysLoading,
    ethernetUp,
    repeaterUp,
    tetherUp,
    wan,
    wanMedium,
    wanProtocol: wanConfig?.type ?? null,
    wifiConn,
    usbTether,
    sysInfo,
    vpnActive,
    ipv6Enabled,
    internetUp,
    allClients,
  };
}
