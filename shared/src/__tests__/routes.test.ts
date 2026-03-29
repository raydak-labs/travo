import { describe, it, expect } from 'vitest';
import { API_ROUTES } from '../api/routes';

function getAllRoutes(obj: Record<string, unknown>, prefix = ''): string[] {
  const routes: string[] = [];
  for (const [key, value] of Object.entries(obj)) {
    if (typeof value === 'string') {
      routes.push(value);
    } else if (typeof value === 'object' && value !== null) {
      routes.push(...getAllRoutes(value as Record<string, unknown>, `${prefix}${key}.`));
    }
  }
  return routes;
}

describe('API_ROUTES', () => {
  it('all routes start with /api/v1/', () => {
    const routes = getAllRoutes(API_ROUTES);
    expect(routes.length).toBeGreaterThan(0);
    for (const route of routes) {
      expect(route).toMatch(/^\/api\/v1\//);
    }
  });

  it('has no duplicate routes', () => {
    const routes = getAllRoutes(API_ROUTES);
    const unique = new Set(routes);
    // saved and deleteSaved share the same base path (different HTTP methods)
    expect(unique.size).toBe(routes.length - 1);
  });

  it('has auth routes matching backend', () => {
    expect(API_ROUTES.auth.login).toBe('/api/v1/auth/login');
    expect(API_ROUTES.auth.logout).toBe('/api/v1/auth/logout');
    expect(API_ROUTES.auth.session).toBe('/api/v1/auth/session');
    expect(API_ROUTES.auth.password).toBe('/api/v1/auth/password');
  });

  it('has system routes matching backend', () => {
    expect(API_ROUTES.system.info).toBe('/api/v1/system/info');
    expect(API_ROUTES.system.stats).toBe('/api/v1/system/stats');
    expect(API_ROUTES.system.logs).toBe('/api/v1/system/logs');
    expect(API_ROUTES.system.kernelLogs).toBe('/api/v1/system/logs/kernel');
    expect(API_ROUTES.system.timezone).toBe('/api/v1/system/timezone');
    expect(API_ROUTES.system.factoryReset).toBe('/api/v1/system/factory-reset');
    expect(API_ROUTES.system.shutdown).toBe('/api/v1/system/shutdown');
    expect(API_ROUTES.system.buttons).toBe('/api/v1/system/buttons');
    expect(API_ROUTES.system.buttonActions).toBe('/api/v1/system/button-actions');
  });

  it('has network routes matching backend', () => {
    expect(API_ROUTES.network.status).toBe('/api/v1/network/status');
    expect(API_ROUTES.network.wan).toBe('/api/v1/network/wan');
    expect(API_ROUTES.network.clients).toBe('/api/v1/network/clients');
    expect(API_ROUTES.network.clientAlias).toBe('/api/v1/network/clients/alias');
    expect(API_ROUTES.network.clientKick).toBe('/api/v1/network/clients/kick');
    expect(API_ROUTES.network.clientBlock).toBe('/api/v1/network/clients/block');
    expect(API_ROUTES.network.clientUnblock).toBe('/api/v1/network/clients/unblock');
    expect(API_ROUTES.network.clientBlocked).toBe('/api/v1/network/clients/blocked');
    expect(API_ROUTES.network.dhcpLeases).toBe('/api/v1/network/dhcp/leases');
    expect(API_ROUTES.network.dhcpReservations).toBe('/api/v1/network/dhcp/reservations');
    expect(API_ROUTES.network.dnsEntries).toBe('/api/v1/network/dns/entries');
    expect(API_ROUTES.network.interfaceState).toBe('/api/v1/network/interfaces/:name/state');
    expect(API_ROUTES.network.ddns).toBe('/api/v1/network/ddns');
    expect(API_ROUTES.network.ddnsStatus).toBe('/api/v1/network/ddns/status');
    expect(API_ROUTES.network.uptimeLog).toBe('/api/v1/network/uptime-log');
    expect(API_ROUTES.network.dataUsage).toBe('/api/v1/network/data-usage');
    expect(API_ROUTES.network.dataUsageReset).toBe('/api/v1/network/data-usage/reset');
    expect(API_ROUTES.network.dataUsageBudget).toBe('/api/v1/network/data-usage/budget');
    expect(API_ROUTES.network.usbTethering).toBe('/api/v1/network/usb-tethering');
    expect(API_ROUTES.network.usbTetheringConfigure).toBe(
      '/api/v1/network/usb-tethering/configure',
    );
    expect(API_ROUTES.network.usbTetheringUnconfigure).toBe(
      '/api/v1/network/usb-tethering/unconfigure',
    );
  });

  it('has wifi routes matching backend', () => {
    expect(API_ROUTES.wifi.scan).toBe('/api/v1/wifi/scan');
    expect(API_ROUTES.wifi.connect).toBe('/api/v1/wifi/connect');
    expect(API_ROUTES.wifi.disconnect).toBe('/api/v1/wifi/disconnect');
    expect(API_ROUTES.wifi.connection).toBe('/api/v1/wifi/connection');
    expect(API_ROUTES.wifi.mode).toBe('/api/v1/wifi/mode');
    expect(API_ROUTES.wifi.saved).toBe('/api/v1/wifi/saved');
    expect(API_ROUTES.wifi.radio).toBe('/api/v1/wifi/radio');
    expect(API_ROUTES.wifi.radios).toBe('/api/v1/wifi/radios');
    expect(API_ROUTES.wifi.radioRole).toBe('/api/v1/wifi/radios/:name/role');
    expect(API_ROUTES.wifi.bandSwitching).toBe('/api/v1/wifi/band-switching');
    expect(API_ROUTES.wifi.mac).toBe('/api/v1/wifi/mac');
    expect(API_ROUTES.wifi.macRandomize).toBe('/api/v1/wifi/mac/randomize');
    expect(API_ROUTES.wifi.guest).toBe('/api/v1/wifi/guest');
    expect(API_ROUTES.wifi.autoreconnect).toBe('/api/v1/wifi/autoreconnect');
    expect(API_ROUTES.wifi.applyConfirm).toBe('/api/v1/wifi/apply/confirm');
    expect(API_ROUTES.wifi.schedule).toBe('/api/v1/wifi/schedule');
    expect(API_ROUTES.wifi.macPolicies).toBe('/api/v1/wifi/mac-policies');
  });

  it('has vpn routes matching backend', () => {
    expect(API_ROUTES.vpn.status).toBe('/api/v1/vpn/status');
    expect(API_ROUTES.vpn.killswitch).toBe('/api/v1/vpn/killswitch');
    expect(API_ROUTES.vpn.wireguard.config).toBe('/api/v1/vpn/wireguard');
    expect(API_ROUTES.vpn.wireguard.toggle).toBe('/api/v1/vpn/wireguard/toggle');
    expect(API_ROUTES.vpn.wireguard.status).toBe('/api/v1/vpn/wireguard/status');
    expect(API_ROUTES.vpn.wireguard.profiles).toBe('/api/v1/vpn/wireguard/profiles');
    expect(API_ROUTES.vpn.wireguard.verify).toBe('/api/v1/vpn/wireguard/verify');
    expect(API_ROUTES.vpn.tailscale.status).toBe('/api/v1/vpn/tailscale');
    expect(API_ROUTES.vpn.tailscale.toggle).toBe('/api/v1/vpn/tailscale/toggle');
    expect(API_ROUTES.vpn.tailscale.auth).toBe('/api/v1/vpn/tailscale/auth');
    expect(API_ROUTES.vpn.tailscale.exitNode).toBe('/api/v1/vpn/tailscale/exit-node');
    expect(API_ROUTES.vpn.dnsLeakTest).toBe('/api/v1/vpn/dns-leak-test');
    expect(API_ROUTES.vpn.speedTest).toBe('/api/v1/vpn/speed-test');
    expect(API_ROUTES.vpn.splitTunnel).toBe('/api/v1/vpn/split-tunnel');
  });

  it('has services routes with :id parameter matching backend', () => {
    expect(API_ROUTES.services.list).toBe('/api/v1/services');
    expect(API_ROUTES.services.install).toBe('/api/v1/services/:id/install');
    expect(API_ROUTES.services.installStream).toBe('/api/v1/services/:id/install/stream');
    expect(API_ROUTES.services.remove).toBe('/api/v1/services/:id/remove');
    expect(API_ROUTES.services.removeStream).toBe('/api/v1/services/:id/remove/stream');
    expect(API_ROUTES.services.start).toBe('/api/v1/services/:id/start');
    expect(API_ROUTES.services.stop).toBe('/api/v1/services/:id/stop');
  });

  it('has captive routes matching backend', () => {
    expect(API_ROUTES.captive.status).toBe('/api/v1/captive/status');
    expect(API_ROUTES.captive.autoAccept).toBe('/api/v1/captive/auto-accept');
  });

  it('covers all backend endpoints', () => {
    // All endpoints registered in backend/internal/api/router.go
    const backendEndpoints = [
      '/api/v1/auth/login',
      '/api/v1/auth/logout',
      '/api/v1/auth/session',
      '/api/v1/auth/password',
      '/api/v1/system/info',
      '/api/v1/system/stats',
      '/api/v1/system/logs',
      '/api/v1/system/logs/kernel',
      '/api/v1/system/reboot',
      '/api/v1/system/shutdown',
      '/api/v1/system/hostname',
      '/api/v1/system/leds',
      '/api/v1/system/leds/schedule',
      '/api/v1/system/timezone',
      '/api/v1/system/backup',
      '/api/v1/system/restore',
      '/api/v1/system/factory-reset',
      '/api/v1/system/firmware/upgrade',
      '/api/v1/system/ntp',
      '/api/v1/system/ntp/sync',
      '/api/v1/system/setup-complete',
      '/api/v1/system/time-sync',
      '/api/v1/system/alerts',
      '/api/v1/network/status',
      '/api/v1/network/wan',
      '/api/v1/network/wan/detect',
      '/api/v1/network/clients',
      '/api/v1/network/clients/alias',
      '/api/v1/network/clients/kick',
      '/api/v1/network/clients/block',
      '/api/v1/network/clients/unblock',
      '/api/v1/network/clients/blocked',
      '/api/v1/network/dhcp',
      '/api/v1/network/dhcp/leases',
      '/api/v1/network/dhcp/reservations',
      '/api/v1/network/dns',
      '/api/v1/network/dns/entries',
      '/api/v1/network/interfaces/:name/state',
      '/api/v1/network/ddns',
      '/api/v1/network/ddns/status',
      '/api/v1/wifi/scan',
      '/api/v1/wifi/connect',
      '/api/v1/wifi/disconnect',
      '/api/v1/wifi/connection',
      '/api/v1/wifi/mode',
      '/api/v1/wifi/saved',
      '/api/v1/wifi/saved/priority',
      '/api/v1/wifi/radio',
      '/api/v1/wifi/radios',
      '/api/v1/wifi/radios/:name/role',
      '/api/v1/wifi/band-switching',
      '/api/v1/wifi/ap',
      '/api/v1/wifi/mac',
      '/api/v1/wifi/mac/randomize',
      '/api/v1/wifi/guest',
      '/api/v1/wifi/autoreconnect',
      '/api/v1/wifi/apply/confirm',
      '/api/v1/vpn/status',
      '/api/v1/vpn/killswitch',
      '/api/v1/vpn/wireguard',
      '/api/v1/vpn/wireguard/toggle',
      '/api/v1/vpn/wireguard/import',
      '/api/v1/vpn/wireguard/status',
      '/api/v1/vpn/wireguard/profiles',
      '/api/v1/vpn/tailscale',
      '/api/v1/vpn/tailscale/toggle',
      '/api/v1/vpn/tailscale/auth',
      '/api/v1/vpn/tailscale/exit-node',
      '/api/v1/services',
      '/api/v1/services/:id/install',
      '/api/v1/services/:id/install/stream',
      '/api/v1/services/:id/remove',
      '/api/v1/services/:id/remove/stream',
      '/api/v1/services/:id/start',
      '/api/v1/services/:id/stop',
      '/api/v1/services/:id/autostart',
      '/api/v1/captive/status',
      '/api/v1/captive/auto-accept',
      '/api/v1/adguard/dns',
      '/api/v1/adguard/config',
      '/api/v1/adguard/password',
      '/api/v1/network/uptime-log',
      '/api/v1/system/buttons',
      '/api/v1/system/button-actions',
      '/api/v1/vpn/dns-leak-test',
      '/api/v1/vpn/speed-test',
      '/api/v1/vpn/wireguard/verify',
      '/api/v1/network/data-usage',
      '/api/v1/network/data-usage/reset',
      '/api/v1/network/data-usage/budget',
      '/api/v1/network/usb-tethering',
      '/api/v1/network/usb-tethering/configure',
      '/api/v1/network/usb-tethering/unconfigure',
      '/api/v1/wifi/schedule',
      '/api/v1/wifi/mac-policies',
      '/api/v1/vpn/split-tunnel',
      '/api/v1/system/alert-thresholds',
      '/api/v1/system/ssh-keys',
      '/api/v1/system/speed-test',
      '/api/v1/network/firewall/zones',
      '/api/v1/network/firewall/port-forwards',
      '/api/v1/network/diagnostics',
      '/api/v1/network/doh',
      '/api/v1/network/ipv6',
      '/api/v1/network/wol',
      '/api/v1/vpn/tailscale/ssh',
    ];

    const definedRoutes = getAllRoutes(API_ROUTES);
    for (const endpoint of backendEndpoints) {
      expect(definedRoutes).toContain(endpoint);
    }
    expect(definedRoutes).toHaveLength(backendEndpoints.length + 1);
  });
});
