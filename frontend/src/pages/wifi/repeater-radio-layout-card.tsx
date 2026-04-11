import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useWifiConnection, useRepeaterRadioReconcile } from '@/hooks/use-wifi';

export function RepeaterRadioLayoutCard() {
  const { data: connection } = useWifiConnection();
  const reconcile = useRepeaterRadioReconcile();
  const isRepeater = connection?.mode === 'repeater';

  if (!isRepeater) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Repeater radio layout</CardTitle>
        <CardDescription>
          Re-apply the default separation: Wi‑Fi uplink (STA) on one radio, downlink access point on
          the other (when both radios are available and “Wi‑Fi on uplink radio” is off). Use this if
          the router ended up with AP and STA on the same radio.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button type="button" disabled={reconcile.isPending} onClick={() => reconcile.mutate()}>
          {reconcile.isPending ? 'Applying…' : 'Re-apply STA/AP separation'}
        </Button>
      </CardContent>
    </Card>
  );
}
