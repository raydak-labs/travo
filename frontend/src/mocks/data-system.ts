/**
 * Mock fixtures for the system routes.
 */
import type {
  SystemInfo,
  SystemStats,
  LogResponse,
  TimezoneConfig,
  NTPConfig,
} from '@shared/index';

export const mockSystemInfo: SystemInfo = {
  hostname: 'GL-MT3000',
  model: 'GL.iNet GL-MT3000 (Beryl AX)',
  firmware_version: 'OpenWrt 23.05.3',
  kernel_version: '5.15.150',
  uptime_seconds: 86432,
};
export const mockSystemStats: SystemStats = {
  cpu: {
    usage_percent: 12.5,
    cores: 2,
    temperature_celsius: 52,
    load_average: [0.15, 0.22, 0.18],
  },
  memory: {
    total_bytes: 536870912,
    used_bytes: 214748365,
    free_bytes: 268435456,
    cached_bytes: 53687091,
    usage_percent: 40,
  },
  storage: {
    total_bytes: 7516192768,
    used_bytes: 1503238554,
    free_bytes: 6012954214,
    usage_percent: 20,
  },
  network: [
    { interface: 'br-lan', rx_bytes: 2147483648, tx_bytes: 1073741824 },
    { interface: 'wwan0', rx_bytes: 1073741824, tx_bytes: 536870912 },
  ],
};

export const mockSystemLogs: LogResponse = {
  source: 'system',
  lines: [
    {
      line: 'Tue Mar 11 09:17:50 2026 daemon.info dnsmasq[1234]: query[A] google.com from 192.168.8.100',
      level: 'info',
    },
    {
      line: 'Tue Mar 11 09:17:51 2026 daemon.info AdGuardHome[3732]: blocked ad.tracker.com',
      level: 'info',
    },
    {
      line: 'Tue Mar 11 09:17:52 2026 daemon.err dnsmasq[1234]: failed to allocate lease',
      level: 'err',
    },
    {
      line: 'Tue Mar 11 09:17:53 2026 kern.warning netifd[456]: Interface wan flapping',
      level: 'warning',
    },
    {
      line: 'Tue Mar 11 09:17:54 2026 daemon.notice hostapd[789]: wlan0: STA aa:bb:cc:dd:ee:ff associated',
      level: 'notice',
    },
    {
      line: 'Tue Mar 11 09:17:55 2026 authpriv.info dropbear[1024]: Password auth succeeded for root',
      level: 'info',
    },
    {
      line: 'Tue Mar 11 09:17:56 2026 daemon.debug wireguard: wg0 peer endpoint 10.0.0.1:51820 handshake',
      level: 'debug',
    },
    {
      line: 'Tue Mar 11 09:17:57 2026 daemon.crit dnsmasq[1234]: unable to bind port 53',
      level: 'crit',
    },
  ],
  total: 8,
};
export const mockKernelLogs: LogResponse = {
  source: 'kernel',
  lines: [{ line: '2026-03-04T12:00:00Z kern.info Kernel initialized', level: 'info' }],
  total: 1,
};
export const mockTimezoneConfig: TimezoneConfig = {
  zonename: 'UTC',
  timezone: 'UTC0',
};
export const mockNTPConfig: NTPConfig = {
  enabled: true,
  servers: [
    '0.openwrt.pool.ntp.org',
    '1.openwrt.pool.ntp.org',
    '2.openwrt.pool.ntp.org',
    '3.openwrt.pool.ntp.org',
  ],
};
