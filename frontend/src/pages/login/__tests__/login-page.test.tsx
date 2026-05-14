import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
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
import { LoginPage } from '../login-page';
import { clearToken } from '@/lib/api-client';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';

function renderLoginPage() {
  clearToken();

  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    component: LoginPage,
  });

  const dashboardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/dashboard',
    component: () => <div data-testid="dashboard">Dashboard</div>,
  });

  const routeTree = rootRoute.addChildren([loginRoute, dashboardRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/login'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

beforeEach(() => {
  clearToken();
});

describe('LoginPage', () => {
  it('renders login form with password input and submit button', async () => {
    renderLoginPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Enter your password')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('renders branding and subtitle', async () => {
    renderLoginPage();
    await waitFor(() => {
      expect(screen.getByText('Travel Router')).toBeInTheDocument();
      expect(screen.getByText(/OpenWrt Travel Router Management/)).toBeInTheDocument();
    });
  });

  it('shows validation when password is empty', async () => {
    const user = userEvent.setup();
    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByText('Password is required')).toBeInTheDocument();
    });
  });

  it('renders remember me checkbox defaulting to checked', async () => {
    renderLoginPage();
    await waitFor(() => {
      const checkbox = screen.getByLabelText(/remember me/i);
      expect(checkbox).toBeInTheDocument();
      expect(checkbox).toBeChecked();
    });
  });

  it('remember me checkbox is toggleable', async () => {
    const user = userEvent.setup();
    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/remember me/i)).toBeInTheDocument();
    });

    const checkbox = screen.getByLabelText(/remember me/i);
    expect(checkbox).toBeChecked();

    await user.click(checkbox);
    expect(checkbox).not.toBeChecked();

    await user.click(checkbox);
    expect(checkbox).toBeChecked();
  });

  it('shows loading state on submit', async () => {
    // Delay the login response so we can observe loading state
    server.use(
      http.post(API_ROUTES.auth.login, async () => {
        await new Promise((resolve) => setTimeout(resolve, 200));
        return HttpResponse.json({
          token: 'mock-jwt-token-abc123',
          expires_at: '2026-03-05T00:00:00Z',
        });
      }),
    );

    const user = userEvent.setup();
    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Enter your password')).toBeInTheDocument();
    });

    await user.type(screen.getByPlaceholderText('Enter your password'), 'admin');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    // Button should show loading state
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled();
    });
  });

  it('shows error on wrong password', async () => {
    const user = userEvent.setup();
    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Enter your password')).toBeInTheDocument();
    });

    await user.type(screen.getByPlaceholderText('Enter your password'), 'wrongpass');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid password');
    });
  });

  it('displays error in a styled alert box with icon', async () => {
    const user = userEvent.setup();
    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Enter your password')).toBeInTheDocument();
    });

    await user.type(screen.getByPlaceholderText('Enter your password'), 'wrongpass');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
      // Alert should be styled with red classes (border, bg)
      expect(alert.className).toContain('border-red');
      expect(alert.className).toContain('bg-red');
    });
  });

  it('redirects to dashboard on successful login', async () => {
    const user = userEvent.setup();
    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Enter your password')).toBeInTheDocument();
    });

    await user.type(screen.getByPlaceholderText('Enter your password'), 'admin');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByTestId('dashboard')).toBeInTheDocument();
    });
  });
});
