import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryCard } from '../query-card';

describe('QueryCard', () => {
  it('shows the skeleton while loading and hides the children', () => {
    const { container } = render(
      <QueryCard isLoading>
        <p>Pool size</p>
      </QueryCard>,
    );

    expect(screen.queryByText('Pool size')).not.toBeInTheDocument();
    expect(container.querySelectorAll('.animate-pulse')).toHaveLength(2);
  });

  it('renders the error message with a retry action instead of the children', async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(
      <QueryCard isLoading={false} isError error={new Error('ubus call failed')} onRetry={onRetry}>
        <p>Pool size</p>
      </QueryCard>,
    );

    expect(screen.getByRole('alert')).toHaveTextContent('ubus call failed');
    expect(screen.queryByText('Pool size')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('falls back to a generic message when the error carries none', () => {
    render(
      <QueryCard isLoading={false} isError>
        <p>Pool size</p>
      </QueryCard>,
    );

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load this data from the router.',
    );
  });

  it('omits the retry button when no handler was given', () => {
    render(
      <QueryCard isLoading={false} isError>
        <p>Pool size</p>
      </QueryCard>,
    );

    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
  });

  it('renders the children only after a successful query', () => {
    render(
      <QueryCard isLoading={false} isError={false}>
        <p>Pool size</p>
      </QueryCard>,
    );

    expect(screen.getByText('Pool size')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
