import { useState } from 'react';
import { Cpu, Radio } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { CardInset } from '@/components/ui/card-inset';
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
import { useRadios, useRepeaterOptions, useSetRadioRole } from '@/hooks/use-wifi';
import { ConfirmRadioDisableDialog } from '@/components/wifi/confirm-radio-disable-dialog';
import { InlineError } from '@/components/ui/inline-error';

/**
 * Why role "both" is refused on multi-radio hardware, and both ways out of it.
 * ADR 0002 §2: an enabled access point and the uplink STA on the same PHY is
 * enough to crash ath11k/IPQ6018, so allow_ap_on_sta_radio is the explicit
 * opt-in. Wording mirrors the backend refusal so the two never disagree.
 */
const BOTH_ROLE_REFUSAL_HINT =
  'Running an access point and the WiFi uplink on the same radio can crash the ' +
  'ath11k chipset. Give the uplink STA its own radio and put the downlink access ' +
  'point on the other one, or enable allow_ap_on_sta_radio in repeater options.';

export function WifiRadioHardwareCard() {
  const { data: radios, isLoading: radiosLoading } = useRadios();
  const { data: repeaterOptions } = useRepeaterOptions();
  const setRadioRole = useSetRadioRole();
  const [pendingDisable, setPendingDisable] = useState<{
    name: string;
    currentRole: string;
  } | null>(null);

  // The backend refuses role "both" whenever the router has more than one
  // radio and allow_ap_on_sta_radio is off — see WifiService.rejectSameRadioAPSTA.
  // Both inputs are already loaded here, so the option is disabled up front
  // instead of letting the operator pick it and read a refusal afterwards.
  const bothRoleRefused =
    (radios?.length ?? 0) >= 2 && repeaterOptions?.allow_ap_on_sta_radio === false;
  const roleError = setRadioRole.error instanceof Error ? setRadioRole.error.message : null;
  const generatedKey = setRadioRole.data?.generated_key;

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
        <CardTitle>Radio Hardware</CardTitle>
        <Cpu className="h-4 w-4 text-gray-500 dark:text-gray-400" />
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
                <CardInset
                  key={radio.name}
                  className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <Radio className="h-4 w-4 shrink-0 text-gray-500 dark:text-gray-400" />
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="min-w-0 text-sm font-medium text-gray-900 dark:text-white">
                          {radio.name}
                        </p>
                        {isRecommended && (
                          <Badge variant="success" className="shrink-0">
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
                        <Badge
                          variant={radio.disabled ? 'destructive' : 'success'}
                          className="shrink-0"
                        >
                          {radio.disabled ? 'Disabled' : 'Active'}
                        </Badge>
                      </div>
                      <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
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
                    <SelectTrigger className="w-full shrink-0 sm:w-32">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="ap">AP only</SelectItem>
                      <SelectItem value="sta">STA only</SelectItem>
                      <SelectItem value="both" disabled={bothRoleRefused}>
                        Both (repeater)
                      </SelectItem>
                      <SelectItem value="none">Disabled</SelectItem>
                    </SelectContent>
                  </Select>
                </CardInset>
              );
            })}
            {bothRoleRefused && (
              <p className="text-xs text-gray-500 dark:text-gray-400">
                <span className="font-medium">Both (repeater) is unavailable.</span>{' '}
                {BOTH_ROLE_REFUSAL_HINT}
              </p>
            )}
            {generatedKey && (
              <div
                role="status"
                className="space-y-1 rounded-md border border-yellow-300 bg-yellow-50 p-3 text-xs text-yellow-900 dark:border-yellow-800 dark:bg-yellow-950 dark:text-yellow-200"
              >
                <p className="font-medium">
                  Generated WiFi password for {setRadioRole.variables?.name} — it is shown only
                  here, copy it now.
                </p>
                <p className="font-mono break-all">{generatedKey}</p>
              </div>
            )}
            {roleError && (
              <InlineError>
                <p className="font-medium">Radio role was not changed.</p>
                <p className="mt-1 text-xs">{roleError}</p>
              </InlineError>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
