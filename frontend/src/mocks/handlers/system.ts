import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import {
  mockKernelLogs,
  mockNTPConfig,
  mockSystemInfo,
  mockSystemLogs,
  mockSystemStats,
  mockTimezoneConfig,
} from '../data';

export const systemHandlers = [
  http.get(API_ROUTES.system.info, () => {
    return HttpResponse.json(mockSystemInfo);
  }),

  http.get(API_ROUTES.system.stats, () => {
    return HttpResponse.json(mockSystemStats);
  }),

  http.get(API_ROUTES.system.statsHistory, () => {
    // Generate 20 mock data points over the last 10 minutes
    const now = Math.floor(Date.now() / 1000);
    const points = Array.from({ length: 20 }, (_, i) => ({
      time: now - (19 - i) * 30,
      cpu: 20 + Math.random() * 40,
      memory: 45 + Math.random() * 15,
      rx_bytes: 1000000 * (i + 1),
      tx_bytes: 500000 * (i + 1),
    }));
    return HttpResponse.json(points);
  }),

  http.get(API_ROUTES.system.logs, ({ request }) => {
    const url = new URL(request.url);
    const service = url.searchParams.get('service');
    const level = url.searchParams.get('level');
    const levelSeverity: Record<string, number> = {
      emerg: 0,
      alert: 1,
      crit: 2,
      err: 3,
      warning: 4,
      notice: 5,
      info: 6,
      debug: 7,
    };
    let lines = [...mockSystemLogs.lines];
    if (service) {
      const lower = service.toLowerCase();
      lines = lines.filter((entry) => entry.line.toLowerCase().includes(lower));
    }
    if (level && level in levelSeverity) {
      const minSev = levelSeverity[level];
      lines = lines.filter((entry) => {
        const entrySev = levelSeverity[entry.level];
        return entrySev !== undefined && entrySev <= minSev;
      });
    }
    return HttpResponse.json({
      source: mockSystemLogs.source,
      lines,
      total: lines.length,
    });
  }),

  http.get(API_ROUTES.system.kernelLogs, () => {
    return HttpResponse.json(mockKernelLogs);
  }),

  http.post(API_ROUTES.system.timeSync, () => {
    return HttpResponse.json({ synced: false, reason: 'clock already accurate' });
  }),

  http.post(API_ROUTES.system.reboot, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.system.shutdown, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.system.ntpSync, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.system.factoryReset, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.system.hostname, async ({ request }) => {
    const body = (await request.json()) as { hostname: string };
    if (!body.hostname) {
      return HttpResponse.json({ error: 'hostname is required' }, { status: 400 });
    }
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.leds, () => {
    return HttpResponse.json({ stealth_mode: false, led_count: 3 });
  }),

  http.get(API_ROUTES.system.ledsSchedule, () => {
    return HttpResponse.json({ enabled: false, on_time: '08:00', off_time: '22:00' });
  }),

  http.put(API_ROUTES.system.ledsSchedule, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.system.leds, async ({ request }) => {
    const body = (await request.json()) as { stealth_mode: boolean };
    return HttpResponse.json({ stealth_mode: body.stealth_mode, led_count: 3 });
  }),

  http.get(API_ROUTES.system.timezone, () => {
    return HttpResponse.json(mockTimezoneConfig);
  }),

  http.put(API_ROUTES.system.timezone, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.ntp, () => {
    return HttpResponse.json(mockNTPConfig);
  }),

  http.put(API_ROUTES.system.ntp, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.backup, () => {
    return new HttpResponse(new Blob(['mock-backup-data'], { type: 'application/gzip' }), {
      headers: { 'Content-Disposition': 'attachment; filename=openwrt-backup.tar.gz' },
    });
  }),

  http.post(API_ROUTES.system.restore, () => {
    return HttpResponse.json({
      status: 'ok',
      message: 'Configuration restored. Reboot to apply changes.',
    });
  }),

  http.post(API_ROUTES.system.firmwareUpgrade, () => {
    return HttpResponse.json({
      status: 'ok',
      message: 'Firmware upgrade initiated. Device will reboot.',
    });
  }),

  // MSW is the development fixture for the normal, already-configured router.
  // Reporting `false` here would route every dev session into the setup wizard
  // and make the rest of the UI unreachable from the mock server.
  http.get(API_ROUTES.system.setupComplete, () => {
    return HttpResponse.json({ complete: true });
  }),

  http.post(API_ROUTES.system.setupComplete, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.alerts, () => {
    return HttpResponse.json({ alerts: [] });
  }),

  http.get(API_ROUTES.system.buttons, () => {
    return HttpResponse.json([
      { name: 'reset', action: 'none' },
      { name: 'wps', action: 'none' },
    ]);
  }),

  http.put(API_ROUTES.system.buttonActions, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.alertThresholds, () => {
    return HttpResponse.json({ storage_percent: 90, cpu_percent: 90, memory_percent: 90 });
  }),

  http.put(API_ROUTES.system.alertThresholds, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.sshKeys, () => {
    return HttpResponse.json({ keys: [] });
  }),

  http.post(API_ROUTES.system.sshKeys, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.delete(`${API_ROUTES.system.sshKeys}/:index`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
