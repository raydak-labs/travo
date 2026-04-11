import { Radio } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useWifiHealth, useRepeaterRadioReconcile } from '@/hooks/use-wifi';

export function WifiRepeaterSameRadioBanner() {
  const { data: health } = useWifiHealth();
  const reconcile = useRepeaterRadioReconcile();

  if (!health?.repeater_same_radio_ap_sta) {
    return null;
  }

  return (
    <div
      role="alert"
      className="flex flex-col gap-3 rounded-lg border border-amber-300 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950 sm:flex-row sm:items-center sm:justify-between"
    >
      <div className="flex gap-3">
        <Radio className="mt-0.5 h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" />
        <div className="space-y-1 text-sm text-amber-900 dark:text-amber-100">
          <p className="font-semibold">Uplink and downlink AP share one radio</p>
          <p className="text-amber-800 dark:text-amber-200">
            The network your devices join (downlink AP) should usually use the other radio than the
            Wi‑Fi uplink; same-radio AP+STA is unstable on many hardware. Prefer the other radio for
            the AP, or enable “Wi‑Fi on uplink radio” in repeater options—not the separate Guest
            Network card in Advanced.
          </p>
        </div>
      </div>
      <Button
        type="button"
        size="sm"
        disabled={reconcile.isPending}
        onClick={() => reconcile.mutate()}
        className="shrink-0"
      >
        {reconcile.isPending ? 'Applying…' : 'Fix radio layout'}
      </Button>
    </div>
  );
}
