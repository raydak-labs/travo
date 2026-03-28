import { useState } from 'react';
import { Shield, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  useFirewallZones,
  usePortForwardRules,
  useAddPortForwardRule,
  useDeletePortForwardRule,
} from '@/hooks/use-network';

function PolicyBadge({ policy }: { policy: string }) {
  const variant =
    policy === 'ACCEPT'
      ? 'success'
      : policy === 'DROP' || policy === 'REJECT'
        ? 'destructive'
        : 'secondary';
  return <Badge variant={variant}>{policy}</Badge>;
}

export function FirewallCard() {
  const { data: zones, isLoading: zonesLoading } = useFirewallZones();
  const { data: rules, isLoading: rulesLoading } = usePortForwardRules();
  const addRule = useAddPortForwardRule();
  const deleteRule = useDeletePortForwardRule();

  const [name, setName] = useState('');
  const [protocol, setProtocol] = useState('tcp');
  const [srcDPort, setSrcDPort] = useState('');
  const [destIP, setDestIP] = useState('');
  const [destPort, setDestPort] = useState('');

  const canAdd = name.trim() !== '' && srcDPort.trim() !== '' && destIP.trim() !== '' && destPort.trim() !== '';

  function handleAdd() {
    if (!canAdd) return;
    addRule.mutate(
      { name: name.trim(), protocol, src_dport: srcDPort.trim(), dest_ip: destIP.trim(), dest_port: destPort.trim(), enabled: true },
      {
        onSuccess: () => {
          setName('');
          setSrcDPort('');
          setDestIP('');
          setDestPort('');
        },
      },
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Firewall</CardTitle>
        <Shield className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Firewall Zones */}
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
            Firewall Zones
          </h3>
          {zonesLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : zones && zones.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-gray-500">
                    <th className="pb-2 font-medium">Zone</th>
                    <th className="pb-2 font-medium">Networks</th>
                    <th className="pb-2 font-medium">Input</th>
                    <th className="pb-2 font-medium">Output</th>
                    <th className="pb-2 font-medium">Forward</th>
                    <th className="pb-2 font-medium">Masq</th>
                  </tr>
                </thead>
                <tbody>
                  {zones.map((zone) => (
                    <tr key={zone.name} className="border-b last:border-0">
                      <td className="py-2 font-medium text-gray-900 dark:text-white">
                        {zone.name}
                      </td>
                      <td className="py-2 text-gray-500">
                        {zone.network && zone.network.length > 0 ? zone.network.join(', ') : '—'}
                      </td>
                      <td className="py-2">
                        <PolicyBadge policy={zone.input} />
                      </td>
                      <td className="py-2">
                        <PolicyBadge policy={zone.output} />
                      </td>
                      <td className="py-2">
                        <PolicyBadge policy={zone.forward} />
                      </td>
                      <td className="py-2">
                        <Badge variant="secondary">—</Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState message="No firewall zones found" />
          )}
        </div>

        {/* Port Forwarding */}
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
            Port Forwarding
          </h3>
          {rulesLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : (
            <div className="space-y-4">
              {rules && rules.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b text-left text-gray-500">
                        <th className="pb-2 font-medium">Name</th>
                        <th className="pb-2 font-medium">Protocol</th>
                        <th className="pb-2 font-medium">Ext. Port</th>
                        <th className="pb-2 font-medium">Internal IP</th>
                        <th className="pb-2 font-medium">Int. Port</th>
                        <th className="w-16 pb-2 font-medium"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {rules.map((rule) => (
                        <tr key={rule.id} className="border-b last:border-0">
                          <td className="py-2 text-gray-900 dark:text-white">{rule.name}</td>
                          <td className="py-2 font-mono uppercase text-gray-500">{rule.protocol}</td>
                          <td className="py-2 font-mono text-gray-900 dark:text-white">
                            {rule.src_dport}
                          </td>
                          <td className="py-2 font-mono text-gray-900 dark:text-white">
                            {rule.dest_ip}
                          </td>
                          <td className="py-2 font-mono text-gray-900 dark:text-white">
                            {rule.dest_port}
                          </td>
                          <td className="py-2 text-right">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => deleteRule.mutate(rule.id)}
                              disabled={deleteRule.isPending}
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

              {/* Add form */}
              <div className="space-y-2">
                <p className="text-xs text-gray-500">Add port forwarding rule</p>
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-[1fr_auto_1fr_1fr_1fr_auto]">
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">Name</label>
                    <Input
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="my-rule"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">Protocol</label>
                    <Select value={protocol} onValueChange={setProtocol}>
                      <SelectTrigger className="w-full lg:w-24">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="tcp">TCP</SelectItem>
                        <SelectItem value="udp">UDP</SelectItem>
                        <SelectItem value="tcp udp">Both</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">External Port</label>
                    <Input
                      value={srcDPort}
                      onChange={(e) => setSrcDPort(e.target.value)}
                      placeholder="8080"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">Internal IP</label>
                    <Input
                      value={destIP}
                      onChange={(e) => setDestIP(e.target.value)}
                      placeholder="192.168.8.10"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">Internal Port</label>
                    <Input
                      value={destPort}
                      onChange={(e) => setDestPort(e.target.value)}
                      placeholder="80"
                    />
                  </div>
                  <Button
                    size="sm"
                    className="self-end"
                    onClick={handleAdd}
                    disabled={addRule.isPending || !canAdd}
                  >
                    {addRule.isPending ? 'Adding…' : 'Add'}
                  </Button>
                </div>
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
