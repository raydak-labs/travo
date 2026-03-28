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
    throw redirect({ to: '/dashboard' });
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

const dashboardRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard',
  component: shellPage('Dashboard', DashboardPage),
});

const wifiRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi',
  component: shellPage('WiFi', WifiPage),
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
    dashboardRoute,
    wifiRoute,
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
