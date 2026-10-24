/**
 * Mock fixtures for the services routes.
 */
import type { ServiceInfo, SQMConfig } from '@shared/index';

export const mockServices: ServiceInfo[] = [
  {
    id: 'tailscale',
    name: 'Tailscale',
    description: 'Zero config VPN mesh network',
    state: 'running',
    version: '1.62.0',
    auto_start: true,
  },
  {
    id: 'adguardhome',
    name: 'AdGuard Home',
    description: 'Network-wide ad and tracker blocker',
    state: 'installed',
    version: '0.107.45',
    auto_start: false,
  },
  {
    id: 'wireguard',
    name: 'WireGuard',
    description: 'Fast, modern, secure VPN tunnel',
    state: 'running',
    version: '1.0.20210914',
    auto_start: true,
  },
  {
    id: 'sqm',
    name: 'SQM (Traffic Shaping)',
    description: 'Smart Queue Management to reduce latency (bufferbloat)',
    state: 'installed',
    auto_start: false,
  },
];
export const mockSQMConfig: SQMConfig = {
  enabled: false,
  interface: 'pppoe-wan',
  download_kbit: 20000,
  upload_kbit: 5000,
  qdisc: 'cake',
  script: 'piece_of_cake.qos',
  advanced_hint: 'Advanced SQM settings are available in LuCI → Network → SQM QoS.',
  detected_uci_section: 'default',
};
