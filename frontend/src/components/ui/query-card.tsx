import type { ReactNode } from 'react';
import { RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { InlineError } from '@/components/ui/inline-error';
import { Skeleton } from '@/components/ui/skeleton';

type QueryCardProps = {
  isLoading: boolean;
  /**
   * True only when the request that feeds this card failed. A card must never
   * fall back to an "absent" claim (no WAN, no clients, not installed) from an
   * undefined value: on a flaky link that reads as a confident statement about
   * the router.
   */
  isError?: boolean;
  error?: unknown;
  onRetry?: () => void;
  /** Replaces the default two-line skeleton. */
  loading?: ReactNode;
  children: ReactNode;
};

const GENERIC_ERROR = 'Could not load this data from the router.';

function errorText(error: unknown): string {
  if (error instanceof Error && error.message.length > 0) {
    return error.message;
  }
  return GENERIC_ERROR;
}

/**
 * Renders a card body from a query: skeleton while loading, an error with a
 * retry action when the request failed, children only once the query actually
 * succeeded.
 */
export function QueryCard({
  isLoading,
  isError = false,
  error,
  onRetry,
  loading,
  children,
}: QueryCardProps) {
  if (isLoading) {
    return (
      <>
        {loading ?? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        )}
      </>
    );
  }

  if (isError) {
    return (
      <div className="space-y-2">
        <InlineError>{errorText(error)}</InlineError>
        {onRetry ? (
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw className="h-3.5 w-3.5" />
            Retry
          </Button>
        ) : null}
      </div>
    );
  }

  return <>{children}</>;
}
