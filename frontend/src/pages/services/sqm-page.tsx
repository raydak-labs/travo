import { Link } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useServices } from '@/hooks/use-services';
import { SQMSection } from '@/pages/services/sqm-section';

export function SQMPage() {
  const { data: services = [], isLoading } = useServices();
  const sqm = services.find((s) => s.id === 'sqm');

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!sqm || sqm.state === 'not_installed') {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">SQM (Traffic Shaping)</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm">
            Install SQM from Installed services, then configure shaping and latency settings here.
          </p>
          <Button asChild variant="secondary">
            <Link to="/services">Go to Installed services</Link>
          </Button>
        </CardContent>
      </Card>
    );
  }

  return <SQMSection sqmService={sqm} />;
}
