import {
  createRouter,
  createRoute,
  createRootRoute,
  redirect,
  Outlet,
} from '@tanstack/react-router';
import { AppShell } from '@/components/layout/app-shell';
import { ErrorBoundary } from '@/components/error-boundary';
import { LoginPage } from '@/pages/login/login-page';
import { DashboardPage } from '@/pages/dashboard/dashboard-page';
import { WifiPage } from '@/pages/wifi/wifi-page';
import { VpnPage } from '@/pages/vpn/vpn-page';
import { ServicesPage } from '@/pages/services/services-page';
import { NetworkPage } from '@/pages/network/network-page';
import { SystemPage } from '@/pages/system/system-page';
import { LogsPage } from '@/pages/logs/logs-page';
import { getToken } from '@/lib/api-client';

// Root route
const rootRoute = createRootRoute({
  component: Outlet,
});

// Public: Login
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
});

// Auth guard helper
function requireAuth() {
  if (!getToken()) {
    throw redirect({ to: '/login' });
  }
}

// Index redirect
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

// Protected layout
const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'protected',
  beforeLoad: requireAuth,
});

// Dashboard
const dashboardRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/dashboard',
  component: () => (
    <AppShell title="Dashboard">
      <ErrorBoundary>
        <DashboardPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

const wifiRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi',
  component: () => (
    <AppShell title="WiFi">
      <ErrorBoundary>
        <WifiPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

const networkRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network',
  component: () => (
    <AppShell title="Network">
      <ErrorBoundary>
        <NetworkPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

const vpnRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/vpn',
  component: () => (
    <AppShell title="VPN">
      <ErrorBoundary>
        <VpnPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

const servicesRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services',
  component: () => (
    <AppShell title="Services">
      <ErrorBoundary>
        <ServicesPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

const systemRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/system',
  component: () => (
    <AppShell title="System">
      <ErrorBoundary>
        <SystemPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

const logsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/logs',
  component: () => (
    <AppShell title="Logs">
      <ErrorBoundary>
        <LogsPage />
      </ErrorBoundary>
    </AppShell>
  ),
});

// Build route tree
const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  protectedRoute.addChildren([
    dashboardRoute,
    wifiRoute,
    networkRoute,
    vpnRoute,
    servicesRoute,
    systemRoute,
    logsRoute,
  ]),
]);

export const router = createRouter({ routeTree });

// Register types
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
