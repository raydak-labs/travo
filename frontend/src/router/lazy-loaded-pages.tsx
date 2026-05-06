import { lazy } from 'react';

export const DashboardPage = lazy(() =>
  import('@/pages/dashboard/dashboard-page').then((m) => ({ default: m.DashboardPage })),
);
export const WifiPage = lazy(() =>
  import('@/pages/wifi/wifi-page').then((m) => ({ default: m.WifiPage })),
);
export const VpnPage = lazy(() =>
  import('@/pages/vpn/vpn-page').then((m) => ({ default: m.VpnPage })),
);
export const ServicesPage = lazy(() =>
  import('@/pages/services/services-page').then((m) => ({ default: m.ServicesPage })),
);
export const TailscalePage = lazy(() =>
  import('@/pages/services/tailscale-page').then((m) => ({ default: m.TailscalePage })),
);
export const SQMPage = lazy(() =>
  import('@/pages/services/sqm-page').then((m) => ({ default: m.SQMPage })),
);
export const SpeedtestPage = lazy(() =>
  import('@/pages/services/speedtest-page').then((m) => ({ default: m.SpeedtestPage })),
);
export const NetworkPage = lazy(() =>
  import('@/pages/network/network-page').then((m) => ({ default: m.NetworkPage })),
);
export const ClientsPage = lazy(() =>
  import('@/pages/clients/clients-page').then((m) => ({ default: m.ClientsPage })),
);
export const SystemPage = lazy(() =>
  import('@/pages/system/system-page').then((m) => ({ default: m.SystemPage })),
);
export const LogsPage = lazy(() =>
  import('@/pages/logs/logs-page').then((m) => ({ default: m.LogsPage })),
);
export const ExperimentalPage = lazy(() =>
  import('@/pages/experimental/experimental-page').then((m) => ({ default: m.ExperimentalPage })),
);
