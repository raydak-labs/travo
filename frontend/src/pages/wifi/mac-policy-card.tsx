import { useState } from 'react';
import { Fingerprint } from 'lucide-react';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useMACPolicies, useSetMACPolicies } from '@/hooks/use-wifi';
import type { MACPolicy } from '@shared/index';
import type { MacPolicyAddFormValues } from '@/lib/schemas/wifi-forms';
import { MACPolicyAddForm } from './mac-policy-add-form';
import { MACPolicyTable } from './mac-policy-table';

export function MACPolicyCard() {
  const { data: macPolicies, isLoading } = useMACPolicies();
  const setMACPolicies = useSetMACPolicies();

  const policies: MACPolicy[] = macPolicies?.policies ? [...macPolicies.policies] : [];

  const [pendingDelete, setPendingDelete] = useState<number | null>(null);

  const onValidAdd = (data: MacPolicyAddFormValues, onSuccess: () => void) => {
    const updated = [...policies, { ssid: data.ssid.trim(), mac: data.mac.trim() }];
    setMACPolicies.mutate({ policies: updated }, { onSuccess });
  };

  function confirmDelete() {
    if (pendingDelete === null) return;
    const updated = policies.filter((_, i) => i !== pendingDelete);
    setMACPolicies.mutate({ policies: updated }, { onSettled: () => setPendingDelete(null) });
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Per-network MAC Policy</CardTitle>
          <Fingerprint className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Per-network MAC Policy</CardTitle>
        <Fingerprint className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Remember which MAC address to use when connecting to specific SSIDs.
        </p>

        {/* Deleting a row used to apply immediately, with no confirmation and
            no undo. */}
        <MACPolicyTable
          policies={policies}
          onDelete={setPendingDelete}
          isPending={setMACPolicies.isPending}
        />

        <ConfirmDialog
          open={pendingDelete !== null}
          onOpenChange={(open) => !open && setPendingDelete(null)}
          title="Remove this MAC rule?"
          description={
            pendingDelete !== null && policies[pendingDelete]
              ? `Network "${policies[pendingDelete].ssid}" will no longer use MAC ${policies[pendingDelete].mac}.`
              : undefined
          }
          confirmLabel="Remove"
          isPending={setMACPolicies.isPending}
          onConfirm={confirmDelete}
        />

        <MACPolicyAddForm onValidSubmit={onValidAdd} isPending={setMACPolicies.isPending} />
      </CardContent>
    </Card>
  );
}
