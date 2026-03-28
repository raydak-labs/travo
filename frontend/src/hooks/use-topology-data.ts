import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Cable, Wifi, Smartphone, Signal } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type { NetworkStatus } from '@shared/index';
import { useNetworkStatus, useIPv6Status } from './use-network';
import { useWifiConnection } from './use-wifi';
import { useVpnStatus } from './use-vpn';
import { useSystemInfo } from './use-system';
import { useUSBTetherStatus } from './use-usb-tether';
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
  // Raw data for ExperimentalPage detail cards
  wan: NetworkStatus['wan'];
  wifiConn: ReturnType<typeof useWifiConnection>['data'];
  usbTether: ReturnType<typeof useUSBTetherStatus>['data'];
  sysInfo: ReturnType<typeof useSystemInfo>['data'];
  vpnActive: boolean;
  ipv6Enabled: boolean;
  internetUp: boolean;
  allClients: NonNullable<NetworkStatus['clients']>;
}

export function useTopologyData(): TopologyData {
  const queryClient = useQueryClient();
  const { subscribe } = useWsSubscribe();

  // staleTime: Infinity prevents HTTP refetches from overwriting WS-fresh data.
  // Other components using useNetworkStatus() without this option are unaffected.
  const { data: network, isLoading: networkLoading } = useNetworkStatus({
    staleTime: Infinity,
  });
  const { data: wifiConn, isLoading: wifiLoading } = useWifiConnection();
  const { data: vpnStatus } = useVpnStatus();
  const { data: sysInfo, isLoading: sysLoading } = useSystemInfo();
  const { data: ipv6Status } = useIPv6Status();
  const { data: usbTether } = useUSBTetherStatus();

  // On network_status WS message, update the React Query cache.
  // All components using useNetworkStatus() benefit automatically.
  useEffect(() => {
    return subscribe('network_status', (raw) => {
      queryClient.setQueryData<NetworkStatus>(['network', 'status'], raw as NetworkStatus);
    });
  }, [subscribe, queryClient]);

  // Connection type derivation — wan.type tells us the actual upstream medium.
  const wan = network?.wan ?? null;
  const ethernetUp = wan?.is_up === true && wan.type !== 'wifi' && wan.type !== 'usb';
  const repeaterUp =
    (wan?.is_up === true && wan.type === 'wifi') ||
    (wifiConn?.connected === true && wifiConn.mode === 'client');
  const tetherUp =
    (wan?.is_up === true && wan.type === 'usb') || (usbTether?.is_up === true);

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
        detail: repeaterUp
          ? (wifiConn?.ssid ?? wan?.ip_address ?? undefined)
          : 'Disabled',
      },
      {
        label: 'USB Tethering',
        icon: Smartphone,
        connected: tetherUp,
        detail: tetherUp
          ? (usbTether?.device_type || wan?.ip_address || 'Connected')
          : 'No device',
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
    wifiConn,
    usbTether,
    sysInfo,
    vpnActive,
    ipv6Enabled,
    internetUp,
    allClients,
  };
}
