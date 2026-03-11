import { useState } from 'react';
import { Pencil, Check, X } from 'lucide-react';
import type { Client } from '@shared/index';
import { formatBytes } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useSetClientAlias } from '@/hooks/use-network';

interface ClientsTableProps {
  clients: readonly Client[];
}

function AliasCell({ client }: { client: Client }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(client.alias ?? '');
  const setAlias = useSetClientAlias();

  const displayName = client.alias || client.hostname || '—';

  const handleSave = () => {
    setAlias.mutate(
      { mac: client.mac_address, alias: value },
      {
        onSuccess: () => setEditing(false),
      },
    );
  };

  const handleCancel = () => {
    setValue(client.alias ?? '');
    setEditing(false);
  };

  if (editing) {
    return (
      <div className="flex items-center gap-1">
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSave();
            if (e.key === 'Escape') handleCancel();
          }}
          className="h-7 w-32 text-sm"
          placeholder="Alias"
          autoFocus
        />
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={handleSave}
          disabled={setAlias.isPending}
        >
          <Check className="h-3 w-3" />
        </Button>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={handleCancel}>
          <X className="h-3 w-3" />
        </Button>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1">
      <div>
        <span className="text-gray-900 dark:text-white">{displayName}</span>
        {client.alias && client.hostname && (
          <span className="ml-1 text-xs text-gray-400">({client.hostname})</span>
        )}
      </div>
      <Button
        variant="ghost"
        size="icon"
        className="h-6 w-6 opacity-0 group-hover:opacity-100"
        onClick={() => setEditing(true)}
      >
        <Pencil className="h-3 w-3" />
      </Button>
    </div>
  );
}

export function ClientsTable({ clients }: ClientsTableProps) {
  const sorted = [...clients].sort((a, b) => {
    const nameA = a.alias || a.hostname || '';
    const nameB = b.alias || b.hostname || '';
    return nameA.localeCompare(nameB);
  });

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-200 dark:border-gray-700">
            <th className="pb-2 text-left font-medium text-gray-500">Name</th>
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
            <tr key={client.mac_address} className="group">
              <td className="py-2">
                <AliasCell client={client} />
              </td>
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
