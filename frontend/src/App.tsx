import { useEffect, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { Toaster } from 'sonner';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { useTheme } from '@/components/layout/use-theme';
import { WsProvider } from '@/lib/ws-context';
import { ApiError, getToken, TOKEN_CHANGE_EVENT } from '@/lib/api-client';
import { useAlertStore } from '@/stores/alert-store';
import { router } from '@/router';

/**
 * A 4xx is an answer, not a hiccup. Retrying it re-sends the request (and, for
 * a 401, re-fires the session teardown a second time), so only transport and
 * 5xx failures are retried.
 */
function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return failureCount < 1;
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: shouldRetry,
    },
  },
});

/**
 * Drops cached server data when a session ends or is replaced.
 *
 * Without this, logging out and back in renders the previous session's
 * queries for up to `staleTime`, and the notification bell keeps the last
 * user's alerts and unread count indefinitely.
 */
function useSessionReset() {
  const client = useQueryClient();

  useEffect(() => {
    let previous = getToken();

    const onTokenChange = () => {
      const next = getToken();
      // Only a session that *ended or was replaced* needs clearing. A first
      // login goes null -> token and has nothing cached to discard.
      if (previous !== null && previous !== next) {
        client.clear();
        useAlertStore.setState({ alerts: [], unreadCount: 0 });
      }
      previous = next;
    };

    window.addEventListener(TOKEN_CHANGE_EVENT, onTokenChange);
    // `storage` covers a logout in another tab, which fires no TOKEN_CHANGE_EVENT.
    window.addEventListener('storage', onTokenChange);
    return () => {
      window.removeEventListener(TOKEN_CHANGE_EVENT, onTokenChange);
      window.removeEventListener('storage', onTokenChange);
    };
  }, [client]);
}

function SessionResetBoundary({ children }: { children: ReactNode }) {
  useSessionReset();
  return children;
}

function ThemedToaster() {
  const { theme } = useTheme();
  return <Toaster theme={theme} position="bottom-right" richColors closeButton />;
}

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <SessionResetBoundary>
          <WsProvider>
            <RouterProvider router={router} />
            <ThemedToaster />
          </WsProvider>
        </SessionResetBoundary>
      </QueryClientProvider>
    </ThemeProvider>
  );
}

export default App;
