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
import { SetupPage } from '@/pages/setup/setup-page';
import { getToken } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

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

// Public: Setup wizard
const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/setup',
  beforeLoad: () => {
    if (!getToken()) {
      throw redirect({ to: '/login' });
    }
  },
  component: SetupPage,
});

// Auth guard helper
function requireAuth() {
  if (!getToken()) {
    throw redirect({ to: '/login' });
  }
}

/** Check setup status and redirect to /setup if not complete */
async function requireSetupComplete() {
  requireAuth();
  try {
    const res = await fetch(API_ROUTES.system.setupComplete, {
      headers: { Authorization: `Bearer ${getToken()}` },
    });
    if (res.ok) {
      const data = (await res.json()) as { complete: boolean };
      if (!data.complete) {
        throw redirect({ to: '/setup' });
      }
    }
  } catch (e: unknown) {
    // Re-throw redirect objects from TanStack Router
    if (e !== null && typeof e === 'object' && 'to' in e) throw e;
    // If the check fails (e.g. network error), allow through
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
  beforeLoad: requireSetupComplete,
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
  setupRoute,
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
