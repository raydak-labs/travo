import { List } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useDHCPLeases } from '@/hooks/use-network';

export function DhcpLeasesCard() {
  const { data: dhcpLeases, isLoading: dhcpLeasesLoading } = useDHCPLeases();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">DHCP Leases</CardTitle>
        <List className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {dhcpLeasesLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : dhcpLeases && dhcpLeases.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-gray-500">
                  <th className="pb-2 font-medium">Hostname</th>
                  <th className="pb-2 font-medium">IP Address</th>
                  <th className="pb-2 font-medium">MAC Address</th>
                  <th className="pb-2 font-medium">Expires</th>
                </tr>
              </thead>
              <tbody>
                {dhcpLeases.map((lease) => (
                  <tr key={lease.mac} className="border-b last:border-0">
                    <td className="py-2 text-gray-900 dark:text-white">{lease.hostname || '—'}</td>
                    <td className="py-2 font-mono text-gray-900 dark:text-white">{lease.ip}</td>
                    <td className="py-2 font-mono text-gray-500">{lease.mac}</td>
                    <td className="py-2 text-gray-500">
                      {new Date(lease.expiry * 1000).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-gray-500">(No active leases)</p>
        )}
      </CardContent>
    </Card>
  );
}
