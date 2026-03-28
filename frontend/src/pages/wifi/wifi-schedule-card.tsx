import { useState, useEffect } from 'react';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { useWiFiSchedule, useSetWiFiSchedule } from '@/hooks/use-wifi';

export function WiFiScheduleCard() {
  const { data: schedule, isLoading } = useWiFiSchedule();
  const setSchedule = useSetWiFiSchedule();

  const [enabled, setEnabled] = useState(false);
  const [onTime, setOnTime] = useState('08:00');
  const [offTime, setOffTime] = useState('22:00');

  useEffect(() => {
    if (schedule) {
      setEnabled(schedule.enabled);
      setOnTime(schedule.on_time || '08:00');
      setOffTime(schedule.off_time || '22:00');
    }
  }, [schedule]);

  const handleSave = () => {
    setSchedule.mutate({ enabled, on_time: onTime, off_time: offTime });
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">WiFi Schedule</CardTitle>
          <Clock className="h-4 w-4 text-gray-400" />
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
        <CardTitle className="text-sm font-medium">WiFi Schedule</CardTitle>
        <Clock className="h-4 w-4 text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <p className="text-sm">Enable schedule</p>
            <p className="text-xs text-gray-500">
              Automatically turn WiFi on and off at set times (cron-based)
            </p>
          </div>
          <Switch
            id="wifi-schedule-toggle"
            label="Enable schedule"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            disabled={setSchedule.isPending}
          />
        </div>

        {enabled && (
          <div className="space-y-3 rounded-md border p-3">
            <div className="flex items-center gap-3">
              <label className="w-16 shrink-0 text-sm text-gray-600 dark:text-gray-400">On at</label>
              <Input
                type="time"
                value={onTime}
                onChange={(e) => setOnTime(e.target.value)}
                className="max-w-[140px]"
              />
            </div>
            <div className="flex items-center gap-3">
              <label className="w-16 shrink-0 text-sm text-gray-600 dark:text-gray-400">Off at</label>
              <Input
                type="time"
                value={offTime}
                onChange={(e) => setOffTime(e.target.value)}
                className="max-w-[140px]"
              />
            </div>
          </div>
        )}

        <Button
          size="sm"
          onClick={handleSave}
          disabled={setSchedule.isPending}
        >
          {setSchedule.isPending ? 'Saving…' : 'Save'}
        </Button>
      </CardContent>
    </Card>
  );
}
