import type { ComponentType } from 'react';
import {
  createRouter,
  createRoute,
  createRootRoute,
  redirect,
  Outlet,
} from '@tanstack/react-router';
import { AppShell } from '@/components/layout/app-shell';
import { LazyPageBoundary } from '@/components/layout/lazy-page-boundary';
import { shellTitleForPath } from '@/components/layout/shell-titles';
import { LoginPage } from '@/pages/login/login-page';
import { SetupPage } from '@/pages/setup/setup-page';
import {
  ClientsPage,
  DashboardPage,
  LogsPage,
  NetworkPage,
  ServicesPage,
  SQMPage,
  SpeedtestPage,
  SystemPage,
  TailscalePage,
  VpnPage,
  WifiPage,
} from '@/router/lazy-loaded-pages';
import { requireAuth, requireSetupComplete } from '@/router/route-guards';

const rootRoute = createRootRoute({
  component: Outlet,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/setup',
  beforeLoad: () => {
    requireAuth();
  },
  component: SetupPage,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'protected',
  beforeLoad: requireSetupComplete,
});

/** Shell header title. WiFi/Network use leaf labels; Services children keep `Services / X`. */
function shellPage(pathname: string, PageComponent: ComponentType) {
  const title = shellTitleForPath(pathname);
  return () => (
    <AppShell title={title}>
      <LazyPageBoundary>
        <PageComponent />
      </LazyPageBoundary>
    </AppShell>
  );
}

const dashboardRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard',
  component: shellPage('/dashboard', DashboardPage),
});

const dashboardV1RedirectRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard-1',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

const dashboardV2RedirectRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard-2',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

const experimentalRedirectRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/experimental',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

const wifiAdvancedRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi/advanced',
  component: shellPage('/wifi/advanced', WifiPage),
});

const wifiRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi',
  component: shellPage('/wifi', WifiPage),
});

const networkConfigurationRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network/configuration',
  component: shellPage('/network/configuration', NetworkPage),
});

const networkAdvancedRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network/advanced',
  component: shellPage('/network/advanced', NetworkPage),
});

const networkRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network',
  component: shellPage('/network', NetworkPage),
});

const clientsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/clients',
  component: shellPage('/clients', ClientsPage),
});

const vpnRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/vpn',
  component: shellPage('/vpn', VpnPage),
});

const servicesRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services',
  component: shellPage('/services', ServicesPage),
});

const tailscaleRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services/tailscale',
  component: shellPage('/services/tailscale', TailscalePage),
});

const sqmRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services/sqm',
  component: shellPage('/services/sqm', SQMPage),
});

const speedtestRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services/speedtest',
  component: shellPage('/services/speedtest', SpeedtestPage),
});

const systemRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/system',
  component: shellPage('/system', SystemPage),
});

const logsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/logs',
  component: shellPage('/logs', LogsPage),
});

const notFoundRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '*',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  setupRoute,
  protectedRoute.addChildren([
    dashboardRoute,
    experimentalRedirectRoute,
    dashboardV1RedirectRoute,
    dashboardV2RedirectRoute,
    wifiAdvancedRoute,
    wifiRoute,
    networkConfigurationRoute,
    networkAdvancedRoute,
    networkRoute,
    clientsRoute,
    vpnRoute,
    servicesRoute,
    tailscaleRoute,
    sqmRoute,
    speedtestRoute,
    systemRoute,
    logsRoute,
  ]),
  notFoundRoute,
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
