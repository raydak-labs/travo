import { useState } from 'react';
import { Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useRunDiagnostics } from '@/hooks/use-network';
import type { DiagnosticsRequest } from '@shared/index';

type DiagType = 'ping' | 'traceroute' | 'dns';

const TABS: { value: DiagType; label: string }[] = [
  { value: 'ping', label: 'Ping' },
  { value: 'traceroute', label: 'Traceroute' },
  { value: 'dns', label: 'DNS' },
];

export function DiagnosticsCard() {
  const [activeTab, setActiveTab] = useState<DiagType>('ping');
  const [target, setTarget] = useState('');
  const runDiagnostics = useRunDiagnostics();

  function handleRun() {
    if (!target.trim()) return;
    const req: DiagnosticsRequest = { type: activeTab, target: target.trim() };
    runDiagnostics.mutate(req);
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Network Diagnostics</CardTitle>
        <Activity className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Tab selector */}
        <div className="flex gap-1 rounded-md border p-1 w-fit">
          {TABS.map((tab) => (
            <button
              key={tab.value}
              onClick={() => {
                setActiveTab(tab.value);
                runDiagnostics.reset();
              }}
              className={`rounded px-3 py-1 text-sm font-medium transition-colors ${
                activeTab === tab.value
                  ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
                  : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Target input + run button */}
        <div className="flex gap-2">
          <Input
            placeholder={activeTab === 'dns' ? 'Hostname (e.g. example.com)' : 'Host or IP address'}
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleRun();
            }}
            className="font-mono text-sm"
          />
          <Button
            onClick={handleRun}
            disabled={!target.trim() || runDiagnostics.isPending}
            size="sm"
          >
            {runDiagnostics.isPending ? 'Running…' : 'Run'}
          </Button>
        </div>

        {/* Output area */}
        {runDiagnostics.isPending && (
          <div className="rounded-md bg-gray-950 p-3 text-xs text-gray-400 dark:bg-gray-900">
            Running {activeTab} to {target}…
          </div>
        )}
        {runDiagnostics.data && (
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md bg-gray-950 p-3 font-mono text-xs text-green-400 dark:bg-gray-900">
            {runDiagnostics.data.output || '(no output)'}
          </pre>
        )}
        {runDiagnostics.isError && (
          <div className="rounded-md border border-red-200 bg-red-50 p-3 text-xs text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
            {runDiagnostics.error.message}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
