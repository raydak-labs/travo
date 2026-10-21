/**
 * Mock fixtures for the network routes.
 */
import type {
  NetworkStatus,
  WanConfig,
  Client,
  DHCPConfig,
  DNSConfig,
  DHCPLease,
  DNSEntry,
  DHCPReservation,
  DDNSConfig,
  DDNSConfigResponse,
  DDNSStatus,
} from '@shared/index';

// Mirrors GET /api/v1/network/status as the router actually answers it:
// `name` is the ubus network section and `type` is the medium/role
// discriminator. In repeater mode the `wan` field carries the wwan (WiFi STA)
// interface, because it is the effective uplink.
export const mockNetworkStatus: NetworkStatus = {
  wan: {
    name: 'wwan',
    type: 'wifi',
    ip_address: '192.168.1.105',
    netmask: '255.255.255.0',
    gateway: '192.168.1.1',
    dns_servers: ['8.8.8.8', '8.8.4.4'],
    mac_address: '00:1A:2B:3C:4D:5E',
    is_up: true,
    rx_bytes: 1073741824,
    tx_bytes: 536870912,
  },
  lan: {
    name: 'lan',
    type: 'lan',
    ip_address: '192.168.8.1',
    netmask: '255.255.255.0',
    gateway: '',
    dns_servers: [],
    mac_address: '00:1A:2B:3C:4D:5F',
    is_up: true,
    rx_bytes: 2147483648,
    tx_bytes: 1073741824,
  },
  interfaces: [
    {
      name: 'wan',
      type: 'wan',
      ip_address: '',
      netmask: '',
      gateway: '',
      dns_servers: [],
      mac_address: '00:1A:2B:3C:4D:60',
      is_up: false,
      rx_bytes: 0,
      tx_bytes: 0,
    },
    {
      name: 'wwan',
      type: 'wifi',
      ip_address: '192.168.1.105',
      netmask: '255.255.255.0',
      gateway: '192.168.1.1',
      dns_servers: ['8.8.8.8', '8.8.4.4'],
      mac_address: '00:1A:2B:3C:4D:5E',
      is_up: true,
      rx_bytes: 1073741824,
      tx_bytes: 536870912,
    },
    {
      name: 'lan',
      type: 'lan',
      ip_address: '192.168.8.1',
      netmask: '255.255.255.0',
      gateway: '',
      dns_servers: [],
      mac_address: '00:1A:2B:3C:4D:5F',
      is_up: true,
      rx_bytes: 2147483648,
      tx_bytes: 1073741824,
    },
  ],
  clients: [
    {
      ip_address: '192.168.8.100',
      mac_address: 'AA:BB:CC:DD:EE:01',
      hostname: 'MacBook-Pro',
      alias: "John's Laptop",
      interface_name: 'br-lan',
      rx_bytes: 524288000,
      tx_bytes: 262144000,
      connected_since: '2026-03-04T08:00:00Z',
    },
    {
      ip_address: '192.168.8.101',
      mac_address: 'AA:BB:CC:DD:EE:02',
      hostname: 'iPhone-15',
      interface_name: 'br-lan',
      rx_bytes: 104857600,
      tx_bytes: 52428800,
      connected_since: '2026-03-04T09:30:00Z',
    },
    {
      ip_address: '192.168.8.102',
      mac_address: 'AA:BB:CC:DD:EE:03',
      hostname: 'iPad-Air',
      alias: 'Living Room iPad',
      interface_name: 'br-lan',
      rx_bytes: 209715200,
      tx_bytes: 104857600,
      connected_since: '2026-03-04T10:00:00Z',
    },
    {
      ip_address: '192.168.8.103',
      mac_address: 'AA:BB:CC:DD:EE:04',
      hostname: 'Pixel-8',
      interface_name: 'br-lan',
      rx_bytes: 41943040,
      tx_bytes: 20971520,
      connected_since: '2026-03-04T11:00:00Z',
    },
    {
      ip_address: '192.168.8.104',
      mac_address: 'AA:BB:CC:DD:EE:05',
      hostname: 'Work-Laptop',
      alias: 'Travel laptop',
      interface_name: 'br-lan',
      rx_bytes: 314572800,
      tx_bytes: 157286400,
      connected_since: '2026-03-04T11:15:00Z',
    },
    {
      ip_address: '192.168.8.105',
      mac_address: 'AA:BB:CC:DD:EE:06',
      hostname: 'Kindle',
      interface_name: 'br-lan',
      rx_bytes: 5242880,
      tx_bytes: 1048576,
      connected_since: '2026-03-04T12:00:00Z',
    },
    {
      ip_address: '192.168.8.106',
      mac_address: 'AA:BB:CC:DD:EE:07',
      hostname: 'Guest-Phone',
      interface_name: 'br-lan',
      rx_bytes: 20971520,
      tx_bytes: 10485760,
      connected_since: '2026-03-04T12:30:00Z',
    },
  ],
  internet_reachable: true,
};
export const mockWanConfig: WanConfig = {
  type: 'dhcp',
  interface_name: 'wlan-sta0',
  ip_address: '192.168.1.105',
  netmask: '255.255.255.0',
  gateway: '192.168.1.1',
  dns_servers: ['8.8.8.8', '8.8.4.4'],
  mtu: 1500,
};
export const mockClients: Client[] = [
  {
    ip_address: '192.168.8.100',
    mac_address: 'AA:BB:CC:DD:EE:01',
    hostname: 'MacBook-Pro',
    alias: "John's Laptop",
    interface_name: 'br-lan',
    rx_bytes: 524288000,
    tx_bytes: 262144000,
    connected_since: '2026-03-04T08:00:00Z',
  },
  {
    ip_address: '192.168.8.101',
    mac_address: 'AA:BB:CC:DD:EE:02',
    hostname: 'iPhone-15',
    interface_name: 'br-lan',
    rx_bytes: 104857600,
    tx_bytes: 52428800,
    connected_since: '2026-03-04T09:30:00Z',
  },
  {
    ip_address: '192.168.8.102',
    mac_address: 'AA:BB:CC:DD:EE:03',
    hostname: 'iPad-Air',
    alias: 'Living Room iPad',
    interface_name: 'br-lan',
    rx_bytes: 209715200,
    tx_bytes: 104857600,
    connected_since: '2026-03-04T10:00:00Z',
  },
  {
    ip_address: '192.168.8.103',
    mac_address: 'AA:BB:CC:DD:EE:04',
    hostname: 'Pixel-8',
    interface_name: 'br-lan',
    rx_bytes: 41943040,
    tx_bytes: 20971520,
    connected_since: '2026-03-04T11:00:00Z',
  },
  {
    ip_address: '192.168.8.104',
    mac_address: 'AA:BB:CC:DD:EE:05',
    hostname: 'Work-Laptop',
    alias: 'Travel laptop',
    interface_name: 'br-lan',
    rx_bytes: 314572800,
    tx_bytes: 157286400,
    connected_since: '2026-03-04T11:15:00Z',
  },
  {
    ip_address: '192.168.8.105',
    mac_address: 'AA:BB:CC:DD:EE:06',
    hostname: 'Kindle',
    interface_name: 'br-lan',
    rx_bytes: 5242880,
    tx_bytes: 1048576,
    connected_since: '2026-03-04T12:00:00Z',
  },
  {
    ip_address: '192.168.8.106',
    mac_address: 'AA:BB:CC:DD:EE:07',
    hostname: 'Guest-Phone',
    interface_name: 'br-lan',
    rx_bytes: 20971520,
    tx_bytes: 10485760,
    connected_since: '2026-03-04T12:30:00Z',
  },
];
export const mockDHCPConfig: DHCPConfig = {
  start: 100,
  limit: 150,
  lease_time: '12h',
};
export const mockDNSConfig: DNSConfig = {
  use_custom_dns: false,
  servers: [],
};
export const mockDHCPLeases: DHCPLease[] = [
  {
    expiry: Math.floor(Date.now() / 1000) + 3600,
    mac: 'aa:bb:cc:dd:ee:ff',
    ip: '192.168.8.100',
    hostname: 'laptop-1',
  },
  {
    expiry: Math.floor(Date.now() / 1000) + 7200,
    mac: '11:22:33:44:55:66',
    ip: '192.168.8.101',
    hostname: 'phone-2',
  },
];
export const mockDNSEntries: DNSEntry[] = [
  {
    name: 'nas',
    ip: '192.168.8.10',
    section: 'dns_nas',
  },
  {
    name: 'printer',
    ip: '192.168.8.20',
    section: 'dns_printer',
  },
];
export const mockDHCPReservations: DHCPReservation[] = [
  {
    name: 'laptop',
    mac: 'AA:BB:CC:DD:EE:01',
    ip: '192.168.8.50',
    section: 'host_laptop',
  },
  {
    name: 'printer',
    mac: '11:22:33:44:55:66',
    ip: '192.168.8.20',
    section: 'host_printer',
  },
];
export const mockBlockedClients: string[] = [];
export const mockDDNSConfig: DDNSConfig = {
  enabled: false,
  service: '',
  domain: '',
  username: '',
  password: '',
  lookup_host: '',
  update_url: '',
};

/** GET /network/ddns returns the config plus whether ddns-scripts is installed. */
export const mockDDNSConfigResponse: DDNSConfigResponse = {
  config: mockDDNSConfig,
  available: true,
};
export const mockDDNSStatus: DDNSStatus = {
  running: false,
  public_ip: '',
  last_update: '',
};
