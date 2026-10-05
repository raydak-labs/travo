import { CardInset } from '@/components/ui/card-inset';
import { Button } from '@/components/ui/button';
import { StatValue } from '@/components/ui/stat-value';

type NtpConfigSummaryViewProps = {
  ntpEnabled: boolean;
  serversSummary: string;
  onSync: () => void;
  onEdit: () => void;
  syncPending: boolean;
  editDisabled: boolean;
};

export function NtpConfigSummaryView({
  ntpEnabled,
  serversSummary,
  onSync,
  onEdit,
  syncPending,
  editDisabled,
}: NtpConfigSummaryViewProps) {
  return (
    <div className="space-y-3">
      <CardInset variant="muted">
        <StatValue label="NTP" value={ntpEnabled ? 'Enabled' : 'Disabled'} />
        <div className="mt-3">
          <StatValue label="Servers" value={<span className="font-mono">{serversSummary}</span>} />
        </div>
      </CardInset>

      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="sm"
          type="button"
          onClick={onSync}
          disabled={syncPending}
          title="Force a one-shot NTP sync with pool.ntp.org"
        >
          {syncPending ? 'Syncing…' : 'Sync Now'}
        </Button>

        <Button
          size="sm"
          type="button"
          onClick={onEdit}
          disabled={editDisabled}
          title="Edit NTP enablement and server list"
        >
          Edit NTP Settings
        </Button>
      </div>
    </div>
  );
}
