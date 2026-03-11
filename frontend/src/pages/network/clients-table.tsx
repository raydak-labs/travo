import type { Client } from '@shared/index';
import { formatBytes } from '@/lib/utils';

interface ClientsTableProps {
  clients: readonly Client[];
}

export function ClientsTable({ clients }: ClientsTableProps) {
  const sorted = [...clients].sort((a, b) => a.hostname.localeCompare(b.hostname));

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-200 dark:border-gray-700">
            <th className="pb-2 text-left font-medium text-gray-500">Hostname</th>
            <th className="pb-2 text-left font-medium text-gray-500">IP</th>
            <th className="hidden pb-2 text-left font-medium text-gray-500 md:table-cell">MAC</th>
            <th className="hidden pb-2 text-left font-medium text-gray-500 lg:table-cell">
              Interface
            </th>
            <th className="hidden pb-2 text-left font-medium text-gray-500 md:table-cell">
              Connected Since
            </th>
            <th className="pb-2 text-right font-medium text-gray-500">Traffic</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
          {sorted.map((client) => (
            <tr key={client.mac_address}>
              <td className="py-2 text-gray-900 dark:text-white">{client.hostname}</td>
              <td className="py-2 text-gray-600 dark:text-gray-400">{client.ip_address}</td>
              <td className="hidden py-2 text-gray-600 dark:text-gray-400 md:table-cell">
                {client.mac_address}
              </td>
              <td className="hidden py-2 text-gray-600 dark:text-gray-400 lg:table-cell">
                {client.interface_name}
              </td>
              <td className="hidden py-2 text-gray-600 dark:text-gray-400 md:table-cell">
                {client.connected_since && !isNaN(new Date(client.connected_since).getTime())
                  ? new Date(client.connected_since).toLocaleString()
                  : '—'}
              </td>
              <td className="py-2 text-right text-gray-600 dark:text-gray-400">
                ↓ {formatBytes(client.rx_bytes)} / ↑ {formatBytes(client.tx_bytes)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
