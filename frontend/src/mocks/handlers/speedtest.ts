import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import type { SpeedTestResult, SpeedtestServiceStatus } from '@shared/index';

const mockSpeedtestServiceStatus: SpeedtestServiceStatus = {
  installed: true,
  supported: true,
  architecture: 'aarch64_cortex-a53',
  version: '1.1.0',
  package_name: 'wget-ssp-speedtest',
  storage_size_mb: 2,
};

const mockSpeedTestResult: SpeedTestResult = {
  download_mbps: 84.2,
  upload_mbps: 21.5,
  ping_ms: 18,
  server: 'Mock ISP, Frankfurt',
};

export const speedtestHandlers = [
  http.post(API_ROUTES.system.speedTest, () => {
    return HttpResponse.json({
      download_mbps: 50.0,
      upload_mbps: 20.0,
      ping_ms: 15.0,
      server: 'test',
    });
  })

  // Routes below had no handler at all. Without them the dev server hit the
  // network for real (main.tsx starts MSW with onUnhandledFrame "bypass") and
  // every test asserted against whatever the machine answered.

  http.get(API_ROUTES.system.speedtestService, () => {
    return HttpResponse.json(mockSpeedtestServiceStatus);
  })

  http.post(API_ROUTES.system.speedtestServiceInstall, () => {
    return HttpResponse.json({ ok: true });
  })

  http.post(API_ROUTES.system.speedtestServiceUninstall, () => {
    return HttpResponse.json({ ok: true });
  })

  http.post(API_ROUTES.system.speedtestServiceRun, () => {
    return HttpResponse.json(mockSpeedTestResult);
  })

  http.post(API_ROUTES.vpn.speedTest, () => {
    return HttpResponse.json(mockSpeedTestResult);
  }),
];
