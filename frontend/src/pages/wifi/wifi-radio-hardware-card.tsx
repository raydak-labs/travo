import { useState } from 'react';
import { Cpu, Radio } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useRadios, useSetRadioRole } from '@/hooks/use-wifi';
import { ConfirmRadioDisableDialog } from '@/components/wifi/confirm-radio-disable-dialog';

export function WifiRadioHardwareCard() {
  const { data: radios, isLoading: radiosLoading } = useRadios();
  const setRadioRole = useSetRadioRole();
  const [pendingDisable, setPendingDisable] = useState<{
    name: string;
    currentRole: string;
  } | null>(null);

  function handleRoleChange(name: string, role: string, currentRole: string) {
    if (role === 'none') {
      setPendingDisable({ name, currentRole });
    } else {
      setRadioRole.mutate({ name, role });
    }
  }

  function handleConfirmDisable() {
    if (pendingDisable) {
      setRadioRole.mutate({ name: pendingDisable.name, role: 'none' });
      setPendingDisable(null);
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Radio Hardware</CardTitle>
        <Cpu className="h-4 w-4 text-gray-400" />
      </CardHeader>
      <CardContent>
        {radiosLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : !radios || radios.length === 0 ? (
          <EmptyState message="No radio hardware detected" />
        ) : (
          <div className="space-y-3">
            {radios.map((radio) => {
              const bandLabel =
                radio.band === '5g'
                  ? '5 GHz'
                  : radio.band === '2g'
                    ? '2.4 GHz'
                    : radio.band === '6g'
                      ? '6 GHz'
                      : radio.band;
              const recommendedRole =
                radio.band === '5g' ? 'ap' : radio.band === '2g' ? 'sta' : null;
              const isRecommended = recommendedRole && radio.role === recommendedRole;
              return (
                <div
                  key={radio.name}
                  className="flex items-center justify-between gap-3 rounded-lg border p-3"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <Radio className="h-4 w-4 shrink-0 text-gray-500" />
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="text-sm font-medium text-gray-900 dark:text-white">
                          {radio.name}
                        </p>
                        {isRecommended && (
                          <Badge className="bg-green-100 text-xs text-green-800 dark:bg-green-900 dark:text-green-200">
                            Recommended
                          </Badge>
                        )}

                        {pendingDisable && (
                          <ConfirmRadioDisableDialog
                            open={true}
                            radioName={pendingDisable.name}
                            isPending={setRadioRole.isPending}
                            onOpenChange={(open) => !open && setPendingDisable(null)}
                            onConfirm={handleConfirmDisable}
                          />
                        )}
                        <Badge variant={radio.disabled ? 'destructive' : 'success'}>
                          {radio.disabled ? 'Disabled' : 'Active'}
                        </Badge>
                      </div>
                      <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500">
                        <span>{bandLabel}</span>
                        <span>Ch {radio.channel}</span>
                        <span>{radio.htmode}</span>
                        <span>{radio.type}</span>
                      </div>
                    </div>
                  </div>
                  <Select
                    value={radio.role}
                    onValueChange={(role) => handleRoleChange(radio.name, role, radio.role)}
                    disabled={setRadioRole.isPending}
                  >
                    <SelectTrigger className="w-32 shrink-0">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="ap">AP only</SelectItem>
                      <SelectItem value="sta">STA only</SelectItem>
                      <SelectItem value="both">Both (repeater)</SelectItem>
                      <SelectItem value="none">Disabled</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
