export interface DNSMode {
  readonly mode: 'default' | 'adguard-forwarding' | 'adguard-direct';
  readonly description: string;
  readonly adguard_running: boolean;
  readonly dns_bypassed: boolean;
}
