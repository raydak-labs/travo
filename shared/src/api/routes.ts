/** API route constants — kept in sync with backend/internal/api/router.go */
export const API_ROUTES = {
  auth: {
    login: '/api/v1/auth/login',
    logout: '/api/v1/auth/logout',
    session: '/api/v1/auth/session',
    password: '/api/v1/auth/password',
  },
  system: {
    info: '/api/v1/system/info',
    stats: '/api/v1/system/stats',
    logs: '/api/v1/system/logs',
    kernelLogs: '/api/v1/system/logs/kernel',
    reboot: '/api/v1/system/reboot',
    hostname: '/api/v1/system/hostname',
    leds: '/api/v1/system/leds',
  },
  network: {
    status: '/api/v1/network/status',
    wan: '/api/v1/network/wan',
    clients: '/api/v1/network/clients',
  },
  wifi: {
    scan: '/api/v1/wifi/scan',
    connect: '/api/v1/wifi/connect',
    disconnect: '/api/v1/wifi/disconnect',
    connection: '/api/v1/wifi/connection',
    mode: '/api/v1/wifi/mode',
    saved: '/api/v1/wifi/saved',
    deleteSaved: '/api/v1/wifi/saved',
  },
  vpn: {
    status: '/api/v1/vpn/status',
    wireguard: {
      config: '/api/v1/vpn/wireguard',
      toggle: '/api/v1/vpn/wireguard/toggle',
      import: '/api/v1/vpn/wireguard/import',
    },
    tailscale: {
      status: '/api/v1/vpn/tailscale',
      toggle: '/api/v1/vpn/tailscale/toggle',
    },
  },
  services: {
    list: '/api/v1/services',
    install: '/api/v1/services/:id/install',
    installStream: '/api/v1/services/:id/install/stream',
    remove: '/api/v1/services/:id/remove',
    removeStream: '/api/v1/services/:id/remove/stream',
    start: '/api/v1/services/:id/start',
    stop: '/api/v1/services/:id/stop',
  },
  captive: {
    status: '/api/v1/captive/status',
  },
} as const;
