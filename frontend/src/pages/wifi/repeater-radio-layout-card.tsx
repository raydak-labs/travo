import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { OperationProgressDialog } from '@/components/ui/operation-progress-dialog';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/ui/empty-state';
import { useWifiConnection, useRepeaterRadioReconcile } from '@/hooks/use-wifi';

export function RepeaterRadioLayoutCard() {
  const { data: connection } = useWifiConnection();
  const reconcile = useRepeaterRadioReconcile();
  const isRepeater = connection?.mode === 'repeater';
  const [confirming, setConfirming] = useState(false);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Repeater radio layout</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Re-apply the default separation: Wi‑Fi uplink (STA) on one radio, downlink access point on
          the other (when both radios are available and “Wi‑Fi on uplink radio” is off). Use this if
          the router ended up with AP and STA on the same radio.
        </p>
        {isRepeater ? (
          <Button type="button" disabled={reconcile.isPending} onClick={() => setConfirming(true)}>
            {reconcile.isPending ? 'Applying…' : 'Re-apply STA/AP separation'}
          </Button>
        ) : (
          <EmptyState message="Available in Travel / repeater mode. Switch Wi‑Fi mode on Connect, then return here." />
        )}
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title="Re-apply STA/AP separation?"
          description="Moves the Wi-Fi uplink (STA) to one radio and the downlink access point to the other, then restarts the wireless subsystem."
          warningText="All Wi-Fi clients disconnect, including this device if you are managing the router over Wi-Fi. Keep this page open until it finishes."
          confirmLabel="Apply"
          isPending={reconcile.isPending}
          onConfirm={() => {
            setConfirming(false);
            reconcile.mutate();
          }}
        />
        {reconcile.isPending && (
          <OperationProgressDialog
            open
            title="Re-applying radio separation"
            description="Keep this page open until the wireless subsystem has restarted."
          />
        )}
      </CardContent>
    </Card>
  );
}
