import { AlertTriangle, AlertCircle } from 'lucide-react';
import { useWifiHealth } from '@/hooks/use-wifi';

function repeaterSameRadioIssuePrefix(issue: string): boolean {
  return issue.startsWith('Repeater:');
}

function warningBannerTitle(issues: readonly string[]): string {
  if (issues.length === 0) {
    return 'Wi‑Fi notice';
  }
  if (issues.length === 1) {
    const i = issues[0];
    if (
      i.includes('no DHCP lease') ||
      (i.includes('lease') && (i.includes('wwan') || i.includes('DHCP')))
    ) {
      return 'Waiting for IP address';
    }
    if (i.includes('not associated')) {
      return 'Wi‑Fi not connected';
    }
  }
  return 'Wi‑Fi notices';
}

export function WifiHealthBanner() {
  const { data } = useWifiHealth();

  if (!data || data.status === 'ok') {
    return null;
  }

  const isError = data.status === 'error';
  const issuesForList = data.repeater_same_radio_ap_sta
    ? data.issues.filter((i) => !repeaterSameRadioIssuePrefix(i))
    : [...data.issues];
  if (!isError && issuesForList.length === 0 && data.repeater_same_radio_ap_sta) {
    return null;
  }
  const issuesForTitle = isError ? data.issues : issuesForList;
  const title = isError ? 'WiFi configuration mismatch' : warningBannerTitle(issuesForTitle);
  const Icon = isError ? AlertCircle : AlertTriangle;
  const containerClasses = isError
    ? 'border-[var(--status-danger-border)] bg-[var(--status-danger-surface)]'
    : 'border-[var(--status-warn-border)] bg-[var(--status-warn-surface)]';
  const iconClasses = isError
    ? 'text-[var(--status-danger-text)]'
    : 'text-[var(--status-warn-text)]';
  const titleClasses = isError
    ? 'text-[var(--status-danger-text)]'
    : 'text-[var(--status-warn-text)]';
  const bodyClasses = isError
    ? 'text-[var(--status-danger-text)]'
    : 'text-[var(--status-warn-text)]';

  return (
    <div role="alert" className={`flex gap-3 rounded-lg border p-4 ${containerClasses}`}>
      <Icon className={`mt-0.5 h-5 w-5 shrink-0 ${iconClasses}`} />
      <div className="flex flex-col gap-2">
        <p className={`text-sm font-semibold ${titleClasses}`}>{title}</p>
        {issuesForList.length > 0 && (
          <ul className={`list-inside list-disc space-y-1 text-sm ${bodyClasses}`}>
            {issuesForList.map((issue) => (
              <li key={issue}>{issue}</li>
            ))}
          </ul>
        )}
        {data.sta && data.wwan && (
          <p className={`text-xs ${bodyClasses}`}>
            STA: <span className="font-mono">{data.sta.ifname}</span> ({data.sta.ssid}) · wwan
            device: <span className="font-mono">{data.wwan.device || '—'}</span>
          </p>
        )}
      </div>
    </div>
  );
}
