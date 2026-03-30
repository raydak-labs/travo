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
import { LoginPage } from '@/pages/login/login-page';
import { SetupPage } from '@/pages/setup/setup-page';
import {
  ClientsPage,
  DashboardPage,
  ExperimentalPage,
  LogsPage,
  NetworkPage,
  ServicesPage,
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
    throw redirect({ to: '/dashboard-2' });
  },
});

const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'protected',
  beforeLoad: requireSetupComplete,
});

function shellPage(title: string, PageComponent: ComponentType) {
  return () => (
    <AppShell title={title}>
      <LazyPageBoundary>
        <PageComponent />
      </LazyPageBoundary>
    </AppShell>
  );
}

const dashboardV1Route = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard-1',
  component: shellPage('Dashboard', DashboardPage),
});

const dashboardV2Route = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard-2',
  component: shellPage('Dashboard (NEW)', ExperimentalPage),
});

const dashboardLegacyRedirectRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard-1' });
  },
});

const experimentalRedirectRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/experimental',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard-2' });
  },
});

const wifiAdvancedRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi/advanced',
  component: shellPage('WiFi', WifiPage),
});

const wifiRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi',
  component: shellPage('WiFi', WifiPage),
});

const networkConfigurationRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network/configuration',
  component: shellPage('Network', NetworkPage),
});

const networkAdvancedRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network/advanced',
  component: shellPage('Network', NetworkPage),
});

const networkRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network',
  component: shellPage('Network', NetworkPage),
});

const clientsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/clients',
  component: shellPage('Clients', ClientsPage),
});

const vpnRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/vpn',
  component: shellPage('VPN', VpnPage),
});

const servicesRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services',
  component: shellPage('Services', ServicesPage),
});

const tailscaleRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services/tailscale',
  component: shellPage('Services / Tailscale', TailscalePage),
});

const systemRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/system',
  component: shellPage('System', SystemPage),
});

const logsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/logs',
  component: shellPage('Logs', LogsPage),
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  setupRoute,
  protectedRoute.addChildren([
    dashboardLegacyRedirectRoute,
    experimentalRedirectRoute,
    dashboardV1Route,
    dashboardV2Route,
    wifiAdvancedRoute,
    wifiRoute,
    networkConfigurationRoute,
    networkAdvancedRoute,
    networkRoute,
    clientsRoute,
    vpnRoute,
    servicesRoute,
    tailscaleRoute,
    systemRoute,
    logsRoute,
  ]),
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
