import { useState } from 'react';
import {
  Users,
  Search,
  Pencil,
  Check,
  X,
  Zap,
  Ban,
  ShieldOff,
  BookmarkPlus,
  Trash2,
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useClients,
  useBlockedClients,
  useSetClientAlias,
  useKickClient,
  useBlockClient,
  useUnblockClient,
  useDHCPReservations,
  useAddDHCPReservation,
  useDeleteDHCPReservation,
} from '@/hooks/use-network';
import { formatBytes } from '@/lib/utils';
import type { Client } from '@shared/index';

function AliasCell({ client }: { client: Client }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(client.alias ?? '');
  const setAlias = useSetClientAlias();

  const displayName = client.alias || client.hostname || '—';

  const handleSave = () => {
    setAlias.mutate({ mac: client.mac_address, alias: value }, { onSuccess: () => setEditing(false) });
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
          className="h-7 w-36 text-sm"
          placeholder="Device alias"
          autoFocus
        />
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={handleSave} disabled={setAlias.isPending}>
          <Check className="h-3 w-3" />
        </Button>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={handleCancel}>
          <X className="h-3 w-3" />
        </Button>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1 group/alias">
      <div>
        <span className="font-medium text-gray-900 dark:text-white">{displayName}</span>
        {client.alias && client.hostname && (
          <span className="ml-1 text-xs text-gray-400">({client.hostname})</span>
        )}
      </div>
      <Button
        variant="ghost"
        size="icon"
        className="h-6 w-6 opacity-0 group-hover/alias:opacity-100"
        title="Edit alias"
        onClick={() => setEditing(true)}
      >
        <Pencil className="h-3 w-3" />
      </Button>
    </div>
  );
}

interface ClientRowProps {
  client: Client;
  isBlocked: boolean;
  hasReservation: boolean;
  onReserveIP: (client: Client) => void;
}

function ClientRow({ client, isBlocked, hasReservation, onReserveIP }: ClientRowProps) {
  const kick = useKickClient();
  const block = useBlockClient();
  const unblock = useUnblockClient();

  const connectedSince =
    client.connected_since && !isNaN(new Date(client.connected_since).getTime())
      ? new Date(client.connected_since).toLocaleString()
      : null;

  return (
    <tr className="group border-b border-gray-100 dark:border-gray-800 last:border-0">
      <td className="py-3 pr-4">
        <AliasCell client={client} />
        <div className="mt-0.5 text-xs text-gray-400">{client.mac_address}</div>
      </td>
      <td className="py-3 pr-4 text-sm text-gray-700 dark:text-gray-300">
        <div className="flex items-center gap-1.5">
          {client.ip_address}
          {hasReservation && (
            <Badge variant="outline" className="text-xs px-1 py-0">static</Badge>
          )}
        </div>
      </td>
      <td className="hidden py-3 pr-4 text-sm text-gray-500 md:table-cell">
        {client.interface_name}
      </td>
      <td className="hidden py-3 pr-4 text-sm text-gray-500 lg:table-cell">
        {connectedSince ?? '—'}
      </td>
      <td className="py-3 pr-4 text-sm text-gray-500">
        <div className="text-xs">
          <span className="text-blue-600 dark:text-blue-400">↓ {formatBytes(client.rx_bytes)}</span>
          {' / '}
          <span className="text-orange-600 dark:text-orange-400">↑ {formatBytes(client.tx_bytes)}</span>
        </div>
      </td>
      <td className="py-3 text-right">
        <div className="flex items-center justify-end gap-1">
          {isBlocked && (
            <Badge variant="destructive" className="mr-1 text-xs">Blocked</Badge>
          )}
          {!hasReservation && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-blue-600"
              title="Reserve this IP (static DHCP)"
              onClick={() => onReserveIP(client)}
            >
              <BookmarkPlus className="h-3.5 w-3.5" />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            title="Kick (disconnect)"
            onClick={() => kick.mutate(client.mac_address)}
            disabled={kick.isPending}
          >
            <Zap className="h-3.5 w-3.5" />
          </Button>
          {isBlocked ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-green-600"
              title="Unblock"
              onClick={() => unblock.mutate(client.mac_address)}
              disabled={unblock.isPending}
            >
              <ShieldOff className="h-3.5 w-3.5" />
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-600"
              title="Block"
              onClick={() => block.mutate(client.mac_address)}
              disabled={block.isPending}
            >
              <Ban className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      </td>
    </tr>
  );
}

export function ClientsPage() {
  const { data: clients, isLoading: clientsLoading } = useClients();
  const { data: blockedMacs } = useBlockedClients();
  const { data: reservations } = useDHCPReservations();
  const addReservation = useAddDHCPReservation();
  const deleteReservation = useDeleteDHCPReservation();

  const [search, setSearch] = useState('');
  const [reserveForm, setReserveForm] = useState<{ mac: string; ip: string; name: string } | null>(null);

  const blockedSet = new Set((blockedMacs ?? []).map((m) => m.toUpperCase()));
  const reservedMacs = new Set((reservations ?? []).map((r) => r.mac.toUpperCase()));

  const filtered = (clients ?? []).filter((c) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (
      c.ip_address.includes(q) ||
      c.mac_address.toLowerCase().includes(q) ||
      (c.hostname ?? '').toLowerCase().includes(q) ||
      (c.alias ?? '').toLowerCase().includes(q)
    );
  });

  const handleReserveIP = (client: Client) => {
    setReserveForm({
      mac: client.mac_address,
      ip: client.ip_address,
      name: client.alias || client.hostname || '',
    });
  };

  const handleSubmitReservation = () => {
    if (!reserveForm) return;
    addReservation.mutate(reserveForm, {
      onSuccess: () => setReserveForm(null),
    });
  };

  return (
    <div className="space-y-4 p-4">
      {/* Connected Clients */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">
            Connected Clients
            {clients && (
              <span className="ml-2 text-xs font-normal text-gray-500">
                ({clients.length} device{clients.length !== 1 ? 's' : ''})
              </span>
            )}
          </CardTitle>
          <Users className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <div className="mb-3 flex items-center gap-2">
            <Search className="h-4 w-4 text-gray-400 shrink-0" />
            <Input
              placeholder="Search by name, IP, or MAC…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8 text-sm"
            />
            {search && (
              <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setSearch('')}>
                <X className="h-4 w-4" />
              </Button>
            )}
          </div>

          {clientsLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : filtered.length === 0 ? (
            <p className="py-4 text-center text-sm text-gray-500">
              {search ? 'No clients match your search.' : 'No clients connected.'}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 dark:border-gray-700 text-left">
                    <th className="pb-2 font-medium text-gray-500">Device</th>
                    <th className="pb-2 font-medium text-gray-500">IP Address</th>
                    <th className="hidden pb-2 font-medium text-gray-500 md:table-cell">Interface</th>
                    <th className="hidden pb-2 font-medium text-gray-500 lg:table-cell">Connected Since</th>
                    <th className="pb-2 font-medium text-gray-500">Traffic</th>
                    <th className="pb-2 text-right font-medium text-gray-500">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((client) => (
                    <ClientRow
                      key={client.mac_address}
                      client={client}
                      isBlocked={blockedSet.has(client.mac_address.toUpperCase())}
                      hasReservation={reservedMacs.has(client.mac_address.toUpperCase())}
                      onReserveIP={handleReserveIP}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Reserve IP inline form */}
          {reserveForm && (
            <div className="mt-4 rounded-md border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950">
              <p className="mb-2 text-sm font-medium text-blue-900 dark:text-blue-200">
                Reserve IP for {reserveForm.mac}
              </p>
              <div className="flex flex-wrap gap-2">
                <Input
                  placeholder="Hostname"
                  value={reserveForm.name}
                  onChange={(e) => setReserveForm({ ...reserveForm, name: e.target.value })}
                  className="h-8 w-40 text-sm"
                />
                <Input
                  placeholder="IP address"
                  value={reserveForm.ip}
                  onChange={(e) => setReserveForm({ ...reserveForm, ip: e.target.value })}
                  className="h-8 w-36 text-sm"
                />
                <Button
                  size="sm"
                  onClick={handleSubmitReservation}
                  disabled={addReservation.isPending || !reserveForm.ip || !reserveForm.name}
                >
                  {addReservation.isPending ? 'Saving…' : 'Save'}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setReserveForm(null)}>
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* DHCP Reservations */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Static IP Reservations</CardTitle>
          <BookmarkPlus className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {!reservations || reservations.length === 0 ? (
            <p className="text-sm text-gray-500">
              No static reservations. Use the bookmark icon next to a connected client to reserve its IP.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 dark:border-gray-700 text-left">
                    <th className="pb-2 font-medium text-gray-500">Hostname</th>
                    <th className="pb-2 font-medium text-gray-500">MAC</th>
                    <th className="pb-2 font-medium text-gray-500">IP</th>
                    <th className="pb-2 text-right font-medium text-gray-500">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {reservations.map((r) => (
                    <tr key={r.section ?? r.mac} className="border-b border-gray-100 dark:border-gray-800 last:border-0">
                      <td className="py-2 pr-4 text-gray-900 dark:text-white">{r.name}</td>
                      <td className="py-2 pr-4 text-gray-500 font-mono text-xs">{r.mac}</td>
                      <td className="py-2 pr-4 text-gray-700 dark:text-gray-300">{r.ip}</td>
                      <td className="py-2 text-right">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-red-600"
                          title="Remove reservation"
                          onClick={() => r.section && deleteReservation.mutate(r.section)}
                          disabled={!r.section || deleteReservation.isPending}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
