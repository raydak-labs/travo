import { Link } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { useServices } from '@/hooks/use-services';
import { SQMSection } from '@/pages/services/sqm-section';

export function SQMPage() {
  const { data, isLoading, isError, error, refetch } = useServices();
  const sqm = data?.find((s) => s.id === 'sqm');

  return (
    <div className="space-y-6">
      {/* A failed service list used to render the "install SQM first" prompt:
          a confident claim, and the wrong instruction, on a bad link. */}
      {sqm && sqm.state !== 'not_installed' ? (
        <SQMSection sqmService={sqm} />
      ) : (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>SQM (Traffic Shaping)</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <QueryCard
              isLoading={isLoading}
              isError={isError}
              error={error}
              onRetry={() => void refetch()}
              loading={
                <div className="space-y-2">
                  <Skeleton className="h-8 w-64" />
                  <Skeleton className="h-64 w-full" />
                </div>
              }
            >
              <p className="text-sm">
                Install SQM from Apps, then configure shaping and latency settings here.
              </p>
              <Button asChild variant="secondary">
                <Link to="/services">Go to Apps</Link>
              </Button>
            </QueryCard>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
