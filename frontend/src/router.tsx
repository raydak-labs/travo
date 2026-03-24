import { lazy, Suspense } from 'react';
import {
  createRouter,
  createRoute,
  createRootRoute,
  redirect,
  Outlet,
} from '@tanstack/react-router';
import { AppShell } from '@/components/layout/app-shell';
import { ErrorBoundary } from '@/components/error-boundary';
import { getToken } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

// Eagerly loaded (always needed)
import { LoginPage } from '@/pages/login/login-page';
import { SetupPage } from '@/pages/setup/setup-page';

// Lazy-loaded pages (loaded only when the route is first visited)
const DashboardPage = lazy(() =>
  import('@/pages/dashboard/dashboard-page').then((m) => ({ default: m.DashboardPage })),
);
const WifiPage = lazy(() =>
  import('@/pages/wifi/wifi-page').then((m) => ({ default: m.WifiPage })),
);
const VpnPage = lazy(() =>
  import('@/pages/vpn/vpn-page').then((m) => ({ default: m.VpnPage })),
);
const ServicesPage = lazy(() =>
  import('@/pages/services/services-page').then((m) => ({ default: m.ServicesPage })),
);
const NetworkPage = lazy(() =>
  import('@/pages/network/network-page').then((m) => ({ default: m.NetworkPage })),
);
const ClientsPage = lazy(() =>
  import('@/pages/clients/clients-page').then((m) => ({ default: m.ClientsPage })),
);
const SystemPage = lazy(() =>
  import('@/pages/system/system-page').then((m) => ({ default: m.SystemPage })),
);
const LogsPage = lazy(() =>
  import('@/pages/logs/logs-page').then((m) => ({ default: m.LogsPage })),
);

/** Wraps a page in ErrorBoundary + Suspense for lazy loading. */
function Page({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <Suspense fallback={<div className="p-8 text-center text-gray-500 text-sm">Loading…</div>}>
        {children}
      </Suspense>
    </ErrorBoundary>
  );
}

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
      <Page>
        <DashboardPage />
      </Page>
    </AppShell>
  ),
});

const wifiRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/wifi',
  component: () => (
    <AppShell title="WiFi">
      <Page>
        <WifiPage />
      </Page>
    </AppShell>
  ),
});

const networkRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/network',
  component: () => (
    <AppShell title="Network">
      <Page>
        <NetworkPage />
      </Page>
    </AppShell>
  ),
});

const clientsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/clients',
  component: () => (
    <AppShell title="Clients">
      <Page>
        <ClientsPage />
      </Page>
    </AppShell>
  ),
});

const vpnRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/vpn',
  component: () => (
    <AppShell title="VPN">
      <Page>
        <VpnPage />
      </Page>
    </AppShell>
  ),
});

const servicesRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/services',
  component: () => (
    <AppShell title="Services">
      <Page>
        <ServicesPage />
      </Page>
    </AppShell>
  ),
});

const systemRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/system',
  component: () => (
    <AppShell title="System">
      <Page>
        <SystemPage />
      </Page>
    </AppShell>
  ),
});

const logsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/logs',
  component: () => (
    <AppShell title="Logs">
      <Page>
        <LogsPage />
      </Page>
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
    clientsRoute,
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
