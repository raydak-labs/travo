import { useQuery } from '@tanstack/react-query';
import { Users } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { apiClient } from '@/lib/api-client';
import type { NetworkStatus } from '@shared/index';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

export function ClientsCard() {
  const { data: network, isLoading } = useQuery({
    queryKey: ['network', 'status'],
    queryFn: () => apiClient.get<NetworkStatus>('/api/v1/network/status'),
  });

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Connected Clients</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-8 w-16" />
        </CardContent>
      </Card>
    );
  }

  const clientCount = network?.clients?.length ?? 0;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Connected Clients</CardTitle>
        <Users className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold text-gray-900 dark:text-white">{clientCount}</div>
        <Link
          to="/network"
          className="mt-2 inline-block text-sm text-blue-600 hover:underline dark:text-blue-400"
        >
          View all clients →
        </Link>
      </CardContent>
    </Card>
  );
}
