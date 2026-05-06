import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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
import {
  useSpeedtestServiceStatus,
  useInstallSpeedtestCLI,
  useUninstallSpeedtestCLI,
  useRunSpeedtest,
} from '@/hooks/use-speedtest';
import { SpeedtestPage } from '../speedtest-page';

vi.mock('@/hooks/use-speedtest', () => ({
  useSpeedtestServiceStatus: vi.fn(),
  useInstallSpeedtestCLI: vi.fn(),
  useUninstallSpeedtestCLI: vi.fn(),
  useRunSpeedtest: vi.fn(),
}));

const mockUseSpeedtestServiceStatus = vi.mocked(useSpeedtestServiceStatus);
const mockUseInstallSpeedtestCLI = vi.mocked(useInstallSpeedtestCLI);
const mockUseUninstallSpeedtestCLI = vi.mocked(useUninstallSpeedtestCLI);
const mockUseRunSpeedtest = vi.mocked(useRunSpeedtest);

function renderSpeedtestPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: '/services/speedtest',
    component: SpeedtestPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ['/services/speedtest'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

const mockStatus = {
  installed: false,
  supported: true,
  architecture: 'aarch64',
  version: '1.0.0',
  package_name: 'speedtest',
  storage_size_mb: 2,
};

describe('SpeedtestPage', () => {
  beforeEach(() => {
    mockUseSpeedtestServiceStatus.mockReturnValue({
      data: mockStatus,
      isLoading: false,
      isError: false,
      error: null,
    } as ReturnType<typeof useSpeedtestServiceStatus>);

    mockUseInstallSpeedtestCLI.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      isSuccess: false,
      isIdle: true,
      reset: vi.fn(),
      status: 'idle',
      voidFn: vi.fn(),
    } as unknown as ReturnType<typeof useInstallSpeedtestCLI>);

    mockUseUninstallSpeedtestCLI.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      isSuccess: false,
      isIdle: true,
      reset: vi.fn(),
      status: 'idle',
      voidFn: vi.fn(),
    } as unknown as ReturnType<typeof useUninstallSpeedtestCLI>);

    mockUseRunSpeedtest.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      isSuccess: false,
      isIdle: true,
      reset: vi.fn(),
      status: 'idle',
      voidFn: vi.fn(),
      data: undefined,
      error: null,
    } as unknown as ReturnType<typeof useRunSpeedtest>);
  });

  it('shows install button when not installed', async () => {
    renderSpeedtestPage();

    await waitFor(() => {
      expect(screen.getByText(/Install speedtest CLI/i)).toBeInTheDocument();
    });
  });

  it('shows uninstall button when installed', async () => {
    mockUseSpeedtestServiceStatus.mockReturnValue({
      data: { ...mockStatus, installed: true },
      isLoading: false,
      isError: false,
      error: null,
    } as ReturnType<typeof useSpeedtestServiceStatus>);

    renderSpeedtestPage();

    await waitFor(() => {
      expect(screen.getByText(/Uninstall/i)).toBeInTheDocument();
    });
  });

  it('shows unsupported warning when architecture not supported', async () => {
    mockUseSpeedtestServiceStatus.mockReturnValue({
      data: { ...mockStatus, supported: false, architecture: 'arm' },
      isLoading: false,
      isError: false,
      error: null,
    } as ReturnType<typeof useSpeedtestServiceStatus>);

    renderSpeedtestPage();

    await waitFor(() => {
      expect(screen.getByText(/not supported/i)).toBeInTheDocument();
    });
  });

  it('shows loading skeletons when loading', async () => {
    mockUseSpeedtestServiceStatus.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    } as ReturnType<typeof useSpeedtestServiceStatus>);

    renderSpeedtestPage();

    await waitFor(() => {
      expect(document.querySelector('.animate-pulse')).toBeInTheDocument();
    });
  });

  it('shows error when status fails to load', async () => {
    mockUseSpeedtestServiceStatus.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('Failed to load'),
    } as ReturnType<typeof useSpeedtestServiceStatus>);

    renderSpeedtestPage();

    await waitFor(() => {
      expect(screen.getByText(/Failed to load speedtest service/i)).toBeInTheDocument();
    });
  });

  it('shows speed test result when available', async () => {
    mockUseRunSpeedtest.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      isSuccess: true,
      isIdle: false,
      reset: vi.fn(),
      status: 'success',
      voidFn: vi.fn(),
      data: {
        download_mbps: 100.5,
        upload_mbps: 50.25,
        ping_ms: 10.5,
        server: 'test-server',
      },
      error: null,
    } as unknown as ReturnType<typeof useRunSpeedtest>);

    mockUseSpeedtestServiceStatus.mockReturnValue({
      data: { ...mockStatus, installed: true },
      isLoading: false,
      isError: false,
      error: null,
    } as ReturnType<typeof useSpeedtestServiceStatus>);

    renderSpeedtestPage();

    await waitFor(() => {
      expect(screen.getByText(/100.50 Mbps/)).toBeInTheDocument();
      expect(screen.getByText(/50.25 Mbps/)).toBeInTheDocument();
      expect(screen.getByText(/10.5 ms/)).toBeInTheDocument();
    });
  });
});