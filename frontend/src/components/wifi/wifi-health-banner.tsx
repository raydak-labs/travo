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
    ? 'border-red-300 bg-red-50 dark:border-red-800 dark:bg-red-950'
    : 'border-amber-300 bg-amber-50 dark:border-amber-800 dark:bg-amber-950';
  const iconClasses = isError
    ? 'text-red-600 dark:text-red-400'
    : 'text-amber-600 dark:text-amber-400';
  const titleClasses = isError
    ? 'text-red-900 dark:text-red-100'
    : 'text-amber-900 dark:text-amber-100';
  const bodyClasses = isError
    ? 'text-red-800 dark:text-red-200'
    : 'text-amber-800 dark:text-amber-200';

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
