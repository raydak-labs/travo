import { Suspense } from 'react';
import { ErrorBoundary } from '@/components/error-boundary';
import { Skeleton } from '@/components/ui/skeleton';

/** Wraps a page in ErrorBoundary + Suspense for lazy loading. */
export function LazyPageBoundary({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <Suspense
        fallback={
          <div className="space-y-4 p-4 sm:p-6" aria-busy="true" aria-label="Loading page">
            <Skeleton className="h-8 w-48 max-w-full" />
            <Skeleton className="h-4 w-full max-w-md" />
            <div className="grid gap-3 sm:grid-cols-2">
              <Skeleton className="h-32 w-full" />
              <Skeleton className="h-32 w-full" />
            </div>
          </div>
        }
      >
        {children}
      </Suspense>
    </ErrorBoundary>
  );
}
