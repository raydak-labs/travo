import { useState, useEffect } from 'react';
import { Network, Globe, Wifi, Settings, List, MapPin, Trash2, HardDrive, Power } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  useNetworkStatus,
  useDHCPConfig,
  useSetDHCPConfig,
  useDNSConfig,
  useSetDNSConfig,
  useDHCPLeases,
  useDNSEntries,
  useAddDNSEntry,
  useDeleteDNSEntry,
  useDHCPReservations,
  useAddDHCPReservation,
  useDeleteDHCPReservation,
  useBlockedClients,
  useSetInterfaceState,
} from '@/hooks/use-network';
import { formatBytes } from '@/lib/utils';
import { ClientsTable } from './clients-table';

export function NetworkPage() {
  const { data: network, isLoading } = useNetworkStatus();
  const { data: dhcpConfig, isLoading: dhcpLoading } = useDHCPConfig();
  const setDHCP = useSetDHCPConfig();
  const { data: dnsConfig, isLoading: dnsLoading } = useDNSConfig();
  const setDNS = useSetDNSConfig();
  const { data: dhcpLeases, isLoading: dhcpLeasesLoading } = useDHCPLeases();
  const { data: dnsEntries, isLoading: dnsEntriesLoading } = useDNSEntries();
  const addDNSEntry = useAddDNSEntry();
  const deleteDNSEntry = useDeleteDNSEntry();
  const { data: dhcpReservations, isLoading: dhcpReservationsLoading } = useDHCPReservations();
  const addDHCPReservation = useAddDHCPReservation();
  const deleteDHCPReservation = useDeleteDHCPReservation();
  const { data: blockedClients } = useBlockedClients();
  const setInterfaceState = useSetInterfaceState();
  const [dhcpStart, setDhcpStart] = useState<number>(100);
  const [dhcpLimit, setDhcpLimit] = useState<number>(150);
  const [dhcpLease, setDhcpLease] = useState<string>('12h');
  const [useCustomDNS, setUseCustomDNS] = useState(false);
  const [dnsServer1, setDnsServer1] = useState('');
  const [dnsServer2, setDnsServer2] = useState('');
  const [newDnsName, setNewDnsName] = useState('');
  const [newDnsIP, setNewDnsIP] = useState('');
  const [newReservationName, setNewReservationName] = useState('');
  const [newReservationMAC, setNewReservationMAC] = useState('');
  const [newReservationIP, setNewReservationIP] = useState('');

  useEffect(() => {
    if (dhcpConfig) {
      setDhcpStart(dhcpConfig.start);
      setDhcpLimit(dhcpConfig.limit);
      setDhcpLease(dhcpConfig.lease_time);
    }
  }, [dhcpConfig]);

  useEffect(() => {
    if (dnsConfig) {
      setUseCustomDNS(dnsConfig.use_custom_dns);
      setDnsServer1(dnsConfig.servers[0] || '');
      setDnsServer2(dnsConfig.servers[1] || '');
    }
  }, [dnsConfig]);

  return (
    <div className="space-y-6">
      {/* Internet Status */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Internet Connectivity</CardTitle>
          <Globe className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-4 w-1/3" />
          ) : (
            <Badge variant={network?.internet_reachable ? 'success' : 'destructive'}>
              {network?.internet_reachable ? 'Connected' : 'No Internet'}
            </Badge>
          )}
        </CardContent>
      </Card>

      {/* WAN Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">WAN Configuration</CardTitle>
          <Wifi className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : network?.wan ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">Type</span>
                <span className="text-gray-900 dark:text-white">{network.wan.type}</span>
                <span className="text-gray-500">IP Address</span>
                <span className="text-gray-900 dark:text-white">{network.wan.ip_address}</span>
                <span className="text-gray-500">Gateway</span>
                <span className="text-gray-900 dark:text-white">{network.wan.gateway}</span>
                <span className="text-gray-500">DNS</span>
                <span className="text-gray-900 dark:text-white">
                  {(network.wan.dns_servers ?? []).join(', ') || '—'}
                </span>
                <span className="text-gray-500">MAC</span>
                <span className="text-gray-900 dark:text-white">{network.wan.mac_address}</span>
                <span className="text-gray-500">Traffic</span>
                <span className="text-gray-900 dark:text-white">
                  ↓ {formatBytes(network.wan.rx_bytes)} / ↑ {formatBytes(network.wan.tx_bytes)}
                </span>
              </div>
            </div>
          ) : (
            <p className="text-sm text-gray-500">WAN not configured</p>
          )}
        </CardContent>
      </Card>

      {/* Network Interfaces */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Network Interfaces</CardTitle>
          <Power className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : network?.interfaces && network.interfaces.length > 0 ? (
            <div className="space-y-3">
              {network.interfaces.map((iface) => (
                <div
                  key={iface.name}
                  className="flex items-center justify-between rounded-md bg-gray-50 p-3 dark:bg-gray-900"
                >
                  <div className="flex items-center gap-3">
                    <Badge variant={iface.is_up ? 'success' : 'secondary'}>
                      {iface.is_up ? 'Up' : 'Down'}
                    </Badge>
                    <div>
                      <span className="text-sm font-medium text-gray-900 dark:text-white">
                        {iface.name.toUpperCase()}
                      </span>
                      {iface.ip_address && (
                        <span className="ml-2 text-xs text-gray-500">{iface.ip_address}</span>
                      )}
                    </div>
                  </div>
                  <Button
                    variant={iface.is_up ? 'destructive' : 'default'}
                    size="sm"
                    onClick={() =>
                      setInterfaceState.mutate({ name: iface.name, up: !iface.is_up })
                    }
                    disabled={setInterfaceState.isPending}
                  >
                    {iface.is_up ? 'Bring Down' : 'Bring Up'}
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500">No interfaces found</p>
          )}
        </CardContent>
      </Card>

      {/* LAN Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">LAN Configuration</CardTitle>
          <Network className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : network ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">IP Address</span>
                <span className="text-gray-900 dark:text-white">{network.lan.ip_address}</span>
                <span className="text-gray-500">Subnet</span>
                <span className="text-gray-900 dark:text-white">{network.lan.netmask}</span>
                <span className="text-gray-500">MAC</span>
                <span className="text-gray-900 dark:text-white">{network.lan.mac_address}</span>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* DHCP Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">DHCP Configuration</CardTitle>
          <Settings className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {dhcpLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Start Offset</label>
                  <Input
                    type="number"
                    min={2}
                    max={254}
                    value={dhcpStart}
                    onChange={(e) => setDhcpStart(Number(e.target.value))}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Pool Size</label>
                  <Input
                    type="number"
                    min={1}
                    max={253}
                    value={dhcpLimit}
                    onChange={(e) => setDhcpLimit(Number(e.target.value))}
                  />
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-xs text-gray-500">Lease Time</label>
                <Select value={dhcpLease} onValueChange={setDhcpLease}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1h">1 hour</SelectItem>
                    <SelectItem value="2h">2 hours</SelectItem>
                    <SelectItem value="6h">6 hours</SelectItem>
                    <SelectItem value="12h">12 hours</SelectItem>
                    <SelectItem value="24h">24 hours</SelectItem>
                    <SelectItem value="48h">48 hours</SelectItem>
                    <SelectItem value="7d">7 days</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button
                onClick={() =>
                  setDHCP.mutate({ start: dhcpStart, limit: dhcpLimit, lease_time: dhcpLease })
                }
                disabled={setDHCP.isPending}
                size="sm"
              >
                {setDHCP.isPending ? 'Saving…' : 'Save DHCP Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* DNS Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">DNS Configuration</CardTitle>
          <Globe className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {dnsLoading ? (
            <Skeleton className="h-4 w-1/2" />
          ) : (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <Switch
                  checked={useCustomDNS}
                  onChange={(e) => setUseCustomDNS(e.target.checked)}
                />
                <span className="text-sm">Use custom DNS servers</span>
              </div>
              {useCustomDNS && (
                <>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDnsServer1('8.8.8.8');
                        setDnsServer2('8.8.4.4');
                      }}
                    >
                      Google
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDnsServer1('1.1.1.1');
                        setDnsServer2('1.0.0.1');
                      }}
                    >
                      Cloudflare
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDnsServer1('9.9.9.9');
                        setDnsServer2('149.112.112.112');
                      }}
                    >
                      Quad9
                    </Button>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <label className="text-xs text-gray-500">Primary DNS</label>
                      <Input
                        value={dnsServer1}
                        onChange={(e) => setDnsServer1(e.target.value)}
                        placeholder="8.8.8.8"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-gray-500">Secondary DNS</label>
                      <Input
                        value={dnsServer2}
                        onChange={(e) => setDnsServer2(e.target.value)}
                        placeholder="8.8.4.4"
                      />
                    </div>
                  </div>
                </>
              )}
              <Button
                size="sm"
                onClick={() => {
                  const servers = [dnsServer1, dnsServer2].filter(Boolean);
                  setDNS.mutate({ use_custom_dns: useCustomDNS, servers });
                }}
                disabled={setDNS.isPending}
              >
                {setDNS.isPending ? 'Saving…' : 'Save DNS Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* DNS Entries */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Local DNS Entries</CardTitle>
          <MapPin className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {dnsEntriesLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : (
            <div className="space-y-4">
              {dnsEntries && dnsEntries.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b text-left text-gray-500">
                        <th className="pb-2 font-medium">Hostname</th>
                        <th className="pb-2 font-medium">IP Address</th>
                        <th className="pb-2 font-medium w-16"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {dnsEntries.map((entry) => (
                        <tr key={entry.section} className="border-b last:border-0">
                          <td className="py-2 text-gray-900 dark:text-white">{entry.name}</td>
                          <td className="py-2 font-mono text-gray-900 dark:text-white">
                            {entry.ip}
                          </td>
                          <td className="py-2 text-right">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => entry.section && deleteDNSEntry.mutate(entry.section)}
                              disabled={deleteDNSEntry.isPending}
                            >
                              <Trash2 className="h-4 w-4 text-red-500" />
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              <div className="grid grid-cols-[1fr_1fr_auto] gap-2 items-end">
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Hostname</label>
                  <Input
                    value={newDnsName}
                    onChange={(e) => setNewDnsName(e.target.value)}
                    placeholder="myserver"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">IP Address</label>
                  <Input
                    value={newDnsIP}
                    onChange={(e) => setNewDnsIP(e.target.value)}
                    placeholder="192.168.8.10"
                  />
                </div>
                <Button
                  size="sm"
                  onClick={() => {
                    if (newDnsName && newDnsIP) {
                      addDNSEntry.mutate(
                        { name: newDnsName, ip: newDnsIP },
                        {
                          onSuccess: () => {
                            setNewDnsName('');
                            setNewDnsIP('');
                          },
                        },
                      );
                    }
                  }}
                  disabled={addDNSEntry.isPending || !newDnsName || !newDnsIP}
                >
                  {addDNSEntry.isPending ? 'Adding…' : 'Add'}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* DHCP Reservations */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">DHCP Reservations</CardTitle>
          <HardDrive className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {dhcpReservationsLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : (
            <div className="space-y-4">
              {dhcpReservations && dhcpReservations.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b text-left text-gray-500">
                        <th className="pb-2 font-medium">Name</th>
                        <th className="pb-2 font-medium">MAC Address</th>
                        <th className="pb-2 font-medium">IP Address</th>
                        <th className="pb-2 font-medium w-16"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {dhcpReservations.map((reservation) => (
                        <tr key={reservation.section} className="border-b last:border-0">
                          <td className="py-2 text-gray-900 dark:text-white">{reservation.name}</td>
                          <td className="py-2 font-mono text-gray-500">{reservation.mac}</td>
                          <td className="py-2 font-mono text-gray-900 dark:text-white">
                            {reservation.ip}
                          </td>
                          <td className="py-2 text-right">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() =>
                                reservation.section &&
                                deleteDHCPReservation.mutate(reservation.section)
                              }
                              disabled={deleteDHCPReservation.isPending}
                            >
                              <Trash2 className="h-4 w-4 text-red-500" />
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2 items-end">
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Name</label>
                  <Input
                    value={newReservationName}
                    onChange={(e) => setNewReservationName(e.target.value)}
                    placeholder="laptop"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">MAC Address</label>
                  <Input
                    value={newReservationMAC}
                    onChange={(e) => setNewReservationMAC(e.target.value)}
                    placeholder="AA:BB:CC:DD:EE:FF"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">IP Address</label>
                  <Input
                    value={newReservationIP}
                    onChange={(e) => setNewReservationIP(e.target.value)}
                    placeholder="192.168.8.50"
                  />
                </div>
                <Button
                  size="sm"
                  onClick={() => {
                    if (newReservationName && newReservationMAC && newReservationIP) {
                      addDHCPReservation.mutate(
                        {
                          name: newReservationName,
                          mac: newReservationMAC,
                          ip: newReservationIP,
                        },
                        {
                          onSuccess: () => {
                            setNewReservationName('');
                            setNewReservationMAC('');
                            setNewReservationIP('');
                          },
                        },
                      );
                    }
                  }}
                  disabled={
                    addDHCPReservation.isPending ||
                    !newReservationName ||
                    !newReservationMAC ||
                    !newReservationIP
                  }
                >
                  {addDHCPReservation.isPending ? 'Adding…' : 'Add'}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* DHCP Leases */}
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
                      <td className="py-2 text-gray-900 dark:text-white">
                        {lease.hostname || '—'}
                      </td>
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

      {/* Connected Clients */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Connected Clients</CardTitle>
          <Network className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : network?.clients && network.clients.length > 0 ? (
            <ClientsTable clients={network.clients} blockedMacs={blockedClients} />
          ) : (
            <p className="text-sm text-gray-500">No clients connected</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
