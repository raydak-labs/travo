import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { Toaster } from 'sonner';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { useTheme } from '@/components/layout/use-theme';
import { WsProvider } from '@/lib/ws-context';
import { router } from '@/router';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

function ThemedToaster() {
  const { theme } = useTheme();
  return <Toaster theme={theme} position="bottom-right" richColors closeButton />;
}

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <RouterProvider router={router} />
          <ThemedToaster />
        </WsProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}

export default App;
