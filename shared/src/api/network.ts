/** Network interface details */
export interface NetworkInterface {
  readonly name: string;
  readonly type: 'wan' | 'lan' | 'wifi' | 'vpn' | 'usb';
  readonly ip_address: string;
  readonly netmask: string;
  readonly gateway: string;
  readonly dns_servers: readonly string[];
  readonly mac_address: string;
  readonly is_up: boolean;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
}

/** WAN connection type */
export type WanType = 'dhcp' | 'static' | 'pppoe' | 'usb_tethering' | 'none';

/** WAN configuration */
export interface WanConfig {
  readonly type: WanType;
  readonly interface_name: string;
  readonly ip_address: string;
  readonly netmask: string;
  readonly gateway: string;
  readonly dns_servers: readonly string[];
  readonly mtu: number;
}

/** Connected client */
export interface Client {
  readonly ip_address: string;
  readonly mac_address: string;
  readonly hostname: string;
  readonly interface_name: string;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
  readonly connected_since: string;
}

/** Overall network status */
export interface NetworkStatus {
  readonly wan: NetworkInterface | null;
  readonly lan: NetworkInterface;
  readonly interfaces: readonly NetworkInterface[];
  readonly clients: readonly Client[];
  readonly internet_reachable: boolean;
}

/** DHCP server configuration for LAN */
export interface DHCPConfig {
  readonly start: number;
  readonly limit: number;
  readonly lease_time: string;
}

/** Custom DNS server configuration */
export interface DNSConfig {
  readonly use_custom_dns: boolean;
  readonly servers: readonly string[];
}

/** Type guard for NetworkStatus */
export function isNetworkStatus(value: unknown): value is NetworkStatus {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    'wan' in v &&
    typeof v.lan === 'object' &&
    v.lan !== null &&
    Array.isArray(v.interfaces) &&
    Array.isArray(v.clients) &&
    typeof v.internet_reachable === 'boolean'
  );
}
