import { CardInset } from '@/components/ui/card-inset';
import { useState } from 'react';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatValue } from '@/components/ui/stat-value';
import { Label } from '@/components/ui/label';
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
  const {
    data: timezoneConfig,
    isLoading: tzLoading,
    isError: tzError,
    error: tzErrorDetail,
    refetch: refetchTz,
  } = useTimezone();
  const setTz = useSetTimezone();
  const [selectedTz, setSelectedTz] = useState<string>('');
  const [editingTimezone, setEditingTimezone] = useState(false);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Time & Timezone</CardTitle>
        <Clock className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        {/* A failed timezone read used to render the same panel as a real one,
            with the value falling back to an em dash. */}
        <QueryCard
          isLoading={tzLoading}
          isError={tzError}
          error={tzErrorDetail}
          onRetry={() => void refetchTz()}
          loading={<Skeleton className="h-4 w-1/2" />}
        >
          <div className="space-y-4">
            <CardInset variant="muted">
              <StatValue label="Timezone" value={timezoneConfig?.zonename ?? null} />
            </CardInset>

            {editingTimezone ? (
              <>
                <div className="space-y-1">
                  <Label htmlFor="system-timezone">Change Timezone</Label>
                  <Select
                    value={selectedTz || timezoneConfig?.zonename || ''}
                    onValueChange={setSelectedTz}
                  >
                    <SelectTrigger id="system-timezone">
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
        </QueryCard>
      </CardContent>
    </Card>
  );
}
