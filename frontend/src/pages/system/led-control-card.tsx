import { useState, useEffect } from 'react';
import { Lightbulb } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { useLEDStatus, useSetLEDStealth, useLEDSchedule, useSetLEDSchedule } from '@/hooks/use-system';

export function LEDControlCard() {
  const { data: ledStatus } = useLEDStatus();
  const setLEDStealthMutation = useSetLEDStealth();
  const { data: ledSchedule } = useLEDSchedule();
  const setLEDScheduleMutation = useSetLEDSchedule();

  const [scheduleEnabled, setScheduleEnabled] = useState(false);
  const [scheduleOnTime, setScheduleOnTime] = useState('07:00');
  const [scheduleOffTime, setScheduleOffTime] = useState('22:00');

  useEffect(() => {
    if (ledSchedule) {
      setScheduleEnabled(ledSchedule.enabled);
      if (ledSchedule.on_time) setScheduleOnTime(ledSchedule.on_time);
      if (ledSchedule.off_time) setScheduleOffTime(ledSchedule.off_time);
    }
  }, [ledSchedule]);

  if (!ledStatus || ledStatus.led_count === 0) return null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">LED Control</CardTitle>
        <Lightbulb className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Stealth Mode Toggle */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-gray-700 dark:text-gray-300">
              {ledStatus.stealth_mode
                ? 'All LEDs are off — stealth mode active'
                : `${ledStatus.led_count} LED${ledStatus.led_count > 1 ? 's' : ''} active`}
            </p>
          </div>
          <Button
            size="sm"
            variant={ledStatus.stealth_mode ? 'default' : 'outline'}
            disabled={setLEDStealthMutation.isPending}
            onClick={() =>
              setLEDStealthMutation.mutate({ stealth_mode: !ledStatus.stealth_mode })
            }
          >
            {ledStatus.stealth_mode ? 'Restore LEDs' : 'Go Stealth'}
          </Button>
        </div>

        {/* Per-LED Status */}
        {ledStatus.leds && ledStatus.leds.length > 0 && (
          <div className="space-y-1">
            <p className="text-xs font-medium text-gray-500">Individual LEDs</p>
            <div className="grid gap-1">
              {ledStatus.leds.map((led) => (
                <div
                  key={led.name}
                  className="flex items-center justify-between rounded-md bg-gray-50 px-3 py-1.5 dark:bg-gray-900"
                >
                  <span className="text-xs text-gray-700 dark:text-gray-300">{led.name}</span>
                  <Badge variant={led.brightness > 0 ? 'success' : 'secondary'}>
                    {led.brightness > 0 ? 'On' : 'Off'}
                  </Badge>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* LED Schedule */}
        <div className="space-y-3 rounded-lg border p-3">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900 dark:text-white">LED Schedule</p>
              <p className="text-xs text-gray-500">
                Automatically turn LEDs on/off at set times
              </p>
            </div>
            <Switch
              id="led-schedule"
              label="Enable schedule"
              checked={scheduleEnabled}
              onChange={(e) => setScheduleEnabled(e.target.checked)}
            />
          </div>
          {scheduleEnabled && (
            <div className="space-y-2">
              <div className="flex items-center gap-3">
                <label
                  htmlFor="led-on-time"
                  className="w-16 text-xs text-gray-600 dark:text-gray-400"
                >
                  LEDs On
                </label>
                <Input
                  id="led-on-time"
                  type="time"
                  value={scheduleOnTime}
                  onChange={(e) => setScheduleOnTime(e.target.value)}
                  className="w-32"
                />
              </div>
              <div className="flex items-center gap-3">
                <label
                  htmlFor="led-off-time"
                  className="w-16 text-xs text-gray-600 dark:text-gray-400"
                >
                  LEDs Off
                </label>
                <Input
                  id="led-off-time"
                  type="time"
                  value={scheduleOffTime}
                  onChange={(e) => setScheduleOffTime(e.target.value)}
                  className="w-32"
                />
              </div>
              <Button
                size="sm"
                disabled={setLEDScheduleMutation.isPending}
                onClick={() =>
                  setLEDScheduleMutation.mutate({
                    enabled: scheduleEnabled,
                    on_time: scheduleOnTime,
                    off_time: scheduleOffTime,
                  })
                }
              >
                {setLEDScheduleMutation.isPending ? 'Saving...' : 'Save Schedule'}
              </Button>
            </div>
          )}
          {!scheduleEnabled && ledSchedule?.enabled && (
            <Button
              size="sm"
              variant="outline"
              disabled={setLEDScheduleMutation.isPending}
              onClick={() =>
                setLEDScheduleMutation.mutate({ enabled: false, on_time: '', off_time: '' })
              }
            >
              Remove Schedule
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
