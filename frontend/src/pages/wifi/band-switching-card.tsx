import { ChevronUp } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { useBandSwitching, useSetBandSwitching } from '@/hooks/use-wifi';

export function BandSwitchingCard() {
  const { data: bandSwitchData } = useBandSwitching();
  const setBandSwitching = useSetBandSwitching();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Auto Band Switching</CardTitle>
        <ChevronUp className="h-4 w-4 text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <p className="text-sm">Automatic band switching</p>
            <p className="text-xs text-gray-500">
              Switches between 2.4 GHz and 5 GHz based on signal quality
            </p>
          </div>
          <Switch
            id="band-switching-toggle"
            label="Enable"
            checked={bandSwitchData?.config.enabled ?? false}
            onChange={(e) => {
              if (bandSwitchData) {
                setBandSwitching.mutate({ ...bandSwitchData.config, enabled: e.target.checked });
              }
            }}
            disabled={setBandSwitching.isPending}
          />
        </div>
        {bandSwitchData?.config.enabled && (
          <div className="rounded-md bg-gray-50 p-3 text-xs dark:bg-gray-900 space-y-2">
            <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-gray-600 dark:text-gray-400">
              <span>Preferred band</span>
              <span className="font-medium text-gray-900 dark:text-white">
                {bandSwitchData.config.preferred_band === '5g' ? '5 GHz' : '2.4 GHz'}
              </span>
              <span>Switch away when below</span>
              <span className="font-mono text-gray-900 dark:text-white">
                {bandSwitchData.config.down_switch_threshold_dbm} dBm for{' '}
                {bandSwitchData.config.down_switch_delay_sec}s
              </span>
              <span>Switch back when above</span>
              <span className="font-mono text-gray-900 dark:text-white">
                {bandSwitchData.config.up_switch_threshold_dbm} dBm for{' '}
                {bandSwitchData.config.up_switch_delay_sec}s
              </span>
            </div>
            {bandSwitchData.status.state !== 'inactive' && (
              <div className="flex items-center gap-2 pt-1 border-t border-gray-200 dark:border-gray-700">
                <span className="text-gray-500">Status:</span>
                <Badge
                  variant={bandSwitchData.status.state === 'monitoring' ? 'success' : 'outline'}
                  className="text-xs"
                >
                  {bandSwitchData.status.state}
                </Badge>
                {bandSwitchData.status.signal_dbm !== 0 && (
                  <span className="font-mono text-gray-700 dark:text-gray-300">
                    {bandSwitchData.status.signal_dbm} dBm ({bandSwitchData.status.current_band})
                  </span>
                )}
                {bandSwitchData.status.weak_signal_secs > 0 && (
                  <span className="text-amber-600">
                    weak {bandSwitchData.status.weak_signal_secs}s /{' '}
                    {bandSwitchData.config.down_switch_delay_sec}s
                  </span>
                )}
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
