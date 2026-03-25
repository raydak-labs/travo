import { useState } from 'react';
import { Fingerprint, Trash2, Plus } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useMACPolicies, useSetMACPolicies } from '@/hooks/use-wifi';
import type { MACPolicy } from '@shared/index';

const MAC_REGEX = /^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$/;

export function MACPolicyCard() {
  const { data: macPolicies, isLoading } = useMACPolicies();
  const setMACPolicies = useSetMACPolicies();

  const [newSSID, setNewSSID] = useState('');
  const [newMAC, setNewMAC] = useState('');
  const [macError, setMacError] = useState('');

  const policies: MACPolicy[] = macPolicies?.policies ? [...macPolicies.policies] : [];

  const handleAdd = () => {
    if (!newSSID.trim()) return;
    if (newMAC && !MAC_REGEX.test(newMAC)) {
      setMacError('Invalid MAC address format (e.g. aa:bb:cc:dd:ee:ff)');
      return;
    }
    setMacError('');
    const updated = [...policies, { ssid: newSSID.trim(), mac: newMAC.trim() }];
    setMACPolicies.mutate(
      { policies: updated },
      {
        onSuccess: () => {
          setNewSSID('');
          setNewMAC('');
        },
      },
    );
  };

  const handleDelete = (index: number) => {
    const updated = policies.filter((_, i) => i !== index);
    setMACPolicies.mutate({ policies: updated });
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Per-network MAC Policy</CardTitle>
          <Fingerprint className="h-4 w-4 text-gray-400" />
        </CardHeader>
        <CardContent>
          <div className="h-16 animate-pulse rounded bg-gray-100 dark:bg-gray-800" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Per-network MAC Policy</CardTitle>
        <Fingerprint className="h-4 w-4 text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-xs text-gray-500">
          Remember which MAC address to use when connecting to specific SSIDs.
        </p>

        {policies.length > 0 ? (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-xs text-gray-500">
                <th className="pb-1 text-left font-medium">SSID</th>
                <th className="pb-1 text-left font-medium">MAC Address</th>
                <th className="pb-1" />
              </tr>
            </thead>
            <tbody>
              {policies.map((policy, i) => (
                <tr key={i} className="border-b last:border-0">
                  <td className="py-1.5 font-mono text-xs">{policy.ssid}</td>
                  <td className="py-1.5 font-mono text-xs text-gray-600 dark:text-gray-400">
                    {policy.mac || <span className="italic text-gray-400">default</span>}
                  </td>
                  <td className="py-1.5 text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDelete(i)}
                      disabled={setMACPolicies.isPending}
                      className="h-6 w-6 p-0 text-red-500 hover:text-red-700"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="text-xs text-gray-400">No policies configured.</p>
        )}

        <div className="space-y-2 rounded-md border p-3">
          <p className="text-xs font-medium text-gray-600 dark:text-gray-400">Add policy</p>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              placeholder="SSID"
              value={newSSID}
              onChange={(e) => setNewSSID(e.target.value)}
              className="flex-1"
            />
            <Input
              placeholder="MAC (aa:bb:cc:dd:ee:ff)"
              value={newMAC}
              onChange={(e) => {
                setNewMAC(e.target.value);
                setMacError('');
              }}
              className="flex-1 font-mono"
            />
            <Button
              size="sm"
              onClick={handleAdd}
              disabled={!newSSID.trim() || setMACPolicies.isPending}
              className="gap-1.5 shrink-0"
            >
              <Plus className="h-3.5 w-3.5" />
              Add
            </Button>
          </div>
          {macError && <p className="text-xs text-red-500">{macError}</p>}
        </div>
      </CardContent>
    </Card>
  );
}
