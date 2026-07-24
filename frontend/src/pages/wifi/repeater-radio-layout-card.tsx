import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/ui/empty-state';
import { useWifiConnection, useRepeaterRadioReconcile } from '@/hooks/use-wifi';

export function RepeaterRadioLayoutCard() {
  const { data: connection } = useWifiConnection();
  const reconcile = useRepeaterRadioReconcile();
  const isRepeater = connection?.mode === 'repeater';

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
          <Button type="button" disabled={reconcile.isPending} onClick={() => reconcile.mutate()}>
            {reconcile.isPending ? 'Applying…' : 'Re-apply STA/AP separation'}
          </Button>
        ) : (
          <EmptyState message="Available in Travel / repeater mode. Switch Wi‑Fi mode on Connect, then return here." />
        )}
      </CardContent>
    </Card>
  );
}
