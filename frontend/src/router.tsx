import type { ComponentType } from 'react';
import {
  createRouter,
  createRoute,
  createRootRoute,
  redirect,
  Outlet,
} from '@tanstack/react-router';
import { AppShell } from '@/components/layout/app-shell';
import { ThemeProvider } from '@/components/layout/theme-provider';
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
import { NotFoundPage } from '@/pages/not-found/not-found-page';
import { TopProgressBar } from '@/components/layout/top-progress-bar';

const rootRoute = createRootRoute({
  component: Outlet,
  // TanStack resolves an unmatched path through `notFoundComponent`, not
  // through a `path: '*'` child of a pathless parent — without this the router
  // rendered its bare "Not Found" text and the styled page below was never
  // reached.
  //
  // Wrapped in AppShell because root-level not-found renders outside the route
  // shell, and a bare card floating on an empty page looks like a crash rather
  // than a wrong address.
  notFoundComponent: () => (
    <ThemeProvider>
      <AppShell title="Not Found">
        <NotFoundPage />
      </AppShell>
    </ThemeProvider>
  ),
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  component: LoginPage,
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/setup',
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
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
]);

export const router = createRouter({
  routeTree,
  // `requireSetupComplete` awaits the setup status on every navigation. With
  // its 30s cache, that guard can leave the UI unresponsive with no feedback.
  defaultPendingMs: 200,
  defaultPendingComponent: TopProgressBar,
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
