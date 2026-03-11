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
  });

  it('has network routes matching backend', () => {
    expect(API_ROUTES.network.status).toBe('/api/v1/network/status');
    expect(API_ROUTES.network.wan).toBe('/api/v1/network/wan');
    expect(API_ROUTES.network.clients).toBe('/api/v1/network/clients');
  });

  it('has wifi routes matching backend', () => {
    expect(API_ROUTES.wifi.scan).toBe('/api/v1/wifi/scan');
    expect(API_ROUTES.wifi.connect).toBe('/api/v1/wifi/connect');
    expect(API_ROUTES.wifi.disconnect).toBe('/api/v1/wifi/disconnect');
    expect(API_ROUTES.wifi.connection).toBe('/api/v1/wifi/connection');
    expect(API_ROUTES.wifi.mode).toBe('/api/v1/wifi/mode');
    expect(API_ROUTES.wifi.saved).toBe('/api/v1/wifi/saved');
  });

  it('has vpn routes matching backend', () => {
    expect(API_ROUTES.vpn.status).toBe('/api/v1/vpn/status');
    expect(API_ROUTES.vpn.wireguard.config).toBe('/api/v1/vpn/wireguard');
    expect(API_ROUTES.vpn.wireguard.toggle).toBe('/api/v1/vpn/wireguard/toggle');
    expect(API_ROUTES.vpn.tailscale.status).toBe('/api/v1/vpn/tailscale');
    expect(API_ROUTES.vpn.tailscale.toggle).toBe('/api/v1/vpn/tailscale/toggle');
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
      '/api/v1/system/hostname',
      '/api/v1/system/leds',
      '/api/v1/system/timezone',
      '/api/v1/network/status',
      '/api/v1/network/wan',
      '/api/v1/network/clients',
      '/api/v1/network/dhcp',
      '/api/v1/wifi/scan',
      '/api/v1/wifi/connect',
      '/api/v1/wifi/disconnect',
      '/api/v1/wifi/connection',
      '/api/v1/wifi/mode',
      '/api/v1/wifi/saved',
      '/api/v1/wifi/ap',
      '/api/v1/vpn/status',
      '/api/v1/vpn/wireguard',
      '/api/v1/vpn/wireguard/toggle',
      '/api/v1/vpn/wireguard/import',
      '/api/v1/vpn/tailscale',
      '/api/v1/vpn/tailscale/toggle',
      '/api/v1/services',
      '/api/v1/services/:id/install',
      '/api/v1/services/:id/install/stream',
      '/api/v1/services/:id/remove',
      '/api/v1/services/:id/remove/stream',
      '/api/v1/services/:id/start',
      '/api/v1/services/:id/stop',
      '/api/v1/captive/status',
    ];

    const definedRoutes = getAllRoutes(API_ROUTES);
    for (const endpoint of backendEndpoints) {
      expect(definedRoutes).toContain(endpoint);
    }
    expect(definedRoutes).toHaveLength(backendEndpoints.length + 1);
  });
});
