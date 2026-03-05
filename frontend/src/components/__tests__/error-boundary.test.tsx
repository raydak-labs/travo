import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ErrorBoundary } from '../error-boundary';

function ThrowingComponent({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error('Test error message');
  }
  return <div>Child content</div>;
}

describe('ErrorBoundary', () => {
  beforeEach(() => {
    // Suppress React error boundary console.error noise
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('renders children when no error is thrown', () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={false} />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Child content')).toBeInTheDocument();
  });

  it('renders fallback UI when an error is thrown', () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByText('An unexpected error occurred. Please try again.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });

  it('shows error details in a collapsible section', () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Error details')).toBeInTheDocument();
    expect(screen.getByText('Test error message')).toBeInTheDocument();
  });

  it('resets the boundary when "Try Again" is clicked', async () => {
    const user = userEvent.setup();
    let shouldThrow = true;

    function ConditionalThrower() {
      if (shouldThrow) throw new Error('boom');
      return <div>Recovered content</div>;
    }

    const { rerender } = render(
      <ErrorBoundary>
        <ConditionalThrower />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();

    // Stop throwing so the next render succeeds
    shouldThrow = false;

    await user.click(screen.getByRole('button', { name: /try again/i }));

    // After reset, the boundary re-renders children
    rerender(
      <ErrorBoundary>
        <ConditionalThrower />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Recovered content')).toBeInTheDocument();
  });

  it('renders a custom fallback when provided', () => {
    render(
      <ErrorBoundary fallback={<div>Custom fallback</div>}>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Custom fallback')).toBeInTheDocument();
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
  });

  it('has an accessible alert role on the fallback UI', () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.getByRole('alert')).toBeInTheDocument();
  });
});
