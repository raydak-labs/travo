import { Fingerprint } from 'lucide-react';
import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { useMACPolicies, useSetMACPolicies } from '@/hooks/use-wifi';
import type { MACPolicy } from '@shared/index';
import type { MacPolicyAddFormValues } from '@/lib/schemas/wifi-forms';
import { MACPolicyAddForm } from './mac-policy-add-form';
import { MACPolicyTable } from './mac-policy-table';

export function MACPolicyCard() {
  const { data: macPolicies, isLoading } = useMACPolicies();
  const setMACPolicies = useSetMACPolicies();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [policyToDelete, setPolicyToDelete] = useState<number | null>(null);

  const policies: MACPolicy[] = macPolicies?.policies ? [...macPolicies.policies] : [];

  const onValidAdd = (data: MacPolicyAddFormValues, onSuccess: () => void) => {
    const updated = [...policies, { ssid: data.ssid.trim(), mac: data.mac.trim() }];
    setMACPolicies.mutate({ policies: updated }, { onSuccess });
  };

  const handleDeleteClick = (index: number) => {
    setPolicyToDelete(index);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = () => {
    if (policyToDelete !== null) {
      const updated = policies.filter((_, i) => i !== policyToDelete);
      setMACPolicies.mutate({ policies: updated });
    }
    setDeleteDialogOpen(false);
    setPolicyToDelete(null);
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

        <MACPolicyTable
          policies={policies}
          onDelete={handleDeleteClick}
          isPending={setMACPolicies.isPending}
        />

        <MACPolicyAddForm onValidSubmit={onValidAdd} isPending={setMACPolicies.isPending} />
      </CardContent>

      <ConfirmDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        title="Remove MAC Policy"
        description="Are you sure you want to remove this MAC address policy? The device will use its default MAC address when connecting to this network."
        warningText="This will remove the custom MAC address configuration for this SSID."
        confirmLabel="Remove Policy"
        isPending={setMACPolicies.isPending}
        onConfirm={handleDeleteConfirm}
      />
    </Card>
  );
}
