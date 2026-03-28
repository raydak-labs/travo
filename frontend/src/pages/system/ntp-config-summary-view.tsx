import { Button } from '@/components/ui/button';

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
      <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
        <div className="flex items-center justify-between">
          <span className="text-gray-500">NTP</span>
          <span className="text-gray-900 dark:text-white">{ntpEnabled ? 'Enabled' : 'Disabled'}</span>
        </div>

        <div className="mt-2">
          <div className="text-xs text-gray-500">Servers</div>
          <div className="mt-1 font-mono text-sm text-gray-900 dark:text-white">{serversSummary}</div>
        </div>
      </div>

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
