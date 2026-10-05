import { describe, it, expect, vi } from 'vitest';
import { createRef } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@/components/layout/theme-provider';
import type { LogResponse } from '@shared/index';
import { LogsTextView } from '../logs-text-view';

function renderView(props: Partial<Parameters<typeof LogsTextView>[0]> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const emptyLogs: LogResponse = { source: 'syslog', lines: [], total: 0 };
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <LogsTextView
          logRef={createRef<HTMLPreElement>()}
          isLoading={false}
          filteredLines={[]}
          lineFilter=""
          logs={emptyLogs}
          {...props}
        />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

// "No log entries" reads as a quiet system. A traveller on a bad link must be
// able to tell that apart from a request that never reached the router.
describe('LogsTextView', () => {
  it('shows the empty state after a successful query that returned nothing', () => {
    renderView();

    expect(screen.getByText('No log entries')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('reports the failure with a retry action when the query errored', async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderView({ isError: true, error: new Error('logd busy'), logs: undefined, onRetry });

    expect(screen.getByRole('alert')).toHaveTextContent('logd busy');
    expect(screen.queryByText('No log entries')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('treats a missing payload without an explicit flag as a failure', () => {
    renderView({ logs: undefined });

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.queryByText('No log entries')).not.toBeInTheDocument();
  });

  it('keeps the "matching filter" wording distinct from a failure', () => {
    renderView({ lineFilter: 'ssh' });

    expect(screen.getByText('No log entries matching filter')).toBeInTheDocument();
  });
});
