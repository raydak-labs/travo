import { useState } from 'react';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useTimezone, useSetTimezone } from '@/hooks/use-system';
import { TIMEZONES } from '@/lib/timezones';

export function SystemTimezoneCard() {
  const { data: timezoneConfig, isLoading: tzLoading } = useTimezone();
  const setTz = useSetTimezone();
  const [selectedTz, setSelectedTz] = useState<string>('');
  const [editingTimezone, setEditingTimezone] = useState(false);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Time & Timezone</CardTitle>
        <Clock className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {tzLoading ? (
          <Skeleton className="h-4 w-1/2" />
        ) : (
          <div className="space-y-4">
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">Timezone</span>
                <span className="text-gray-900 dark:text-white">
                  {timezoneConfig?.zonename || '—'}
                </span>
              </div>
            </div>

            {editingTimezone ? (
              <>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Change Timezone</label>
                  <Select
                    value={selectedTz || timezoneConfig?.zonename || ''}
                    onValueChange={setSelectedTz}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select timezone" />
                    </SelectTrigger>
                    <SelectContent>
                      {TIMEZONES.map((tz) => (
                        <SelectItem key={tz.zonename} value={tz.zonename}>
                          {tz.zonename}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="flex flex-wrap gap-2">
                  <Button
                    onClick={() => {
                      const tz = TIMEZONES.find((t) => t.zonename === selectedTz);
                      if (tz) {
                        setTz.mutate(
                          { zonename: tz.zonename, timezone: tz.timezone },
                          { onSuccess: () => setEditingTimezone(false) },
                        );
                      }
                    }}
                    disabled={setTz.isPending || !selectedTz}
                    size="sm"
                  >
                    {setTz.isPending ? 'Saving…' : 'Save Timezone'}
                  </Button>

                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      if (timezoneConfig?.zonename) setSelectedTz(timezoneConfig.zonename);
                      setEditingTimezone(false);
                    }}
                    disabled={setTz.isPending}
                  >
                    Cancel
                  </Button>
                </div>
              </>
            ) : (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  if (timezoneConfig?.zonename) setSelectedTz(timezoneConfig.zonename);
                  setEditingTimezone(true);
                }}
                disabled={!timezoneConfig?.zonename || setTz.isPending}
              >
                Edit Timezone
              </Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
