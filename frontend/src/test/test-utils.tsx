import { type ReactNode } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createRouter,
  createRoute,
  createRootRoute,
  RouterProvider,
  Outlet,
  createMemoryHistory,
} from '@tanstack/react-router';
import { ThemeProvider } from '@/components/layout/theme-provider';

interface WrapperOptions {
  initialPath?: string;
  routes?: Array<{ path: string; component: () => ReactNode }>;
}

function createTestRouter(opts: WrapperOptions = {}) {
  const { initialPath = '/', routes = [] } = opts;

  const rootRoute = createRootRoute({ component: Outlet });

  const childRoutes = routes.map((r) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path: r.path,
      component: r.component as () => ReactNode,
    }),
  );

  const routeTree = rootRoute.addChildren(childRoutes);

  return createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });
}

export function renderWithProviders(
  ui: ReactNode,
  {
    initialPath = '/test',
    renderOptions,
  }: {
    initialPath?: string;
    renderOptions?: Omit<RenderOptions, 'wrapper'>;
  } = {},
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  const TestComponent = () => <>{ui}</>;

  const router = createTestRouter({
    initialPath,
    routes: [
      { path: initialPath, component: TestComponent },
      { path: '/dashboard', component: () => <div>Dashboard</div> },
      { path: '/login', component: () => <div>Login</div> },
    ],
  });

  function Wrapper() {
    return (
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </ThemeProvider>
    );
  }

  return render(<Wrapper />, renderOptions);
}
