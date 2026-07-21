import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { InlineError } from '@/components/ui/inline-error';
import { cn } from '@/lib/cn';
import { useRunDiagnostics } from '@/hooks/use-network';
import { diagnosticsFormSchema, type DiagnosticsFormValues } from '@/lib/schemas/network-forms';

type DiagType = 'ping' | 'traceroute' | 'dns';

const TABS: { value: DiagType; label: string }[] = [
  { value: 'ping', label: 'Ping' },
  { value: 'traceroute', label: 'Traceroute' },
  { value: 'dns', label: 'DNS' },
];

export function DiagnosticsCard() {
  const runDiagnostics = useRunDiagnostics();

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useForm<DiagnosticsFormValues>({
    resolver: zodResolver(diagnosticsFormSchema),
    defaultValues: { type: 'ping', target: '' },
    mode: 'onChange',
  });

  const activeTab = watch('type');
  const targetValue = watch('target');

  const onRun = (data: DiagnosticsFormValues) => {
    runDiagnostics.mutate({ type: data.type, target: data.target.trim() });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Network Diagnostics</CardTitle>
        <Activity className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onRun)} className="space-y-4" noValidate>
          <div
            role="tablist"
            aria-label="Diagnostics type"
            className="flex w-fit flex-wrap gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-white/10 dark:bg-gray-900/40"
          >
            {TABS.map((tab) => (
              <button
                key={tab.value}
                type="button"
                role="tab"
                aria-selected={activeTab === tab.value}
                onClick={() => {
                  setValue('type', tab.value, { shouldValidate: false });
                  runDiagnostics.reset();
                }}
                className={cn(
                  'rounded-md px-3 py-1.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
                  activeTab === tab.value
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white'
                    : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white',
                )}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="flex gap-2">
            <Input
              placeholder={
                activeTab === 'dns' ? 'Hostname (e.g. example.com)' : 'Host or IP address'
              }
              className="font-mono text-sm"
              aria-invalid={errors.target ? 'true' : undefined}
              aria-describedby={errors.target ? 'diag-target-err' : undefined}
              {...register('target')}
            />
            <Button type="submit" disabled={!targetValue?.trim() || runDiagnostics.isPending}>
              {runDiagnostics.isPending ? 'Running…' : 'Run'}
            </Button>
          </div>
          {errors.target ? (
            <p id="diag-target-err" className="text-xs text-red-500" role="alert">
              {errors.target.message}
            </p>
          ) : null}

          {runDiagnostics.isPending && (
            <div className="rounded-md bg-gray-950 p-3 text-xs text-gray-400 dark:bg-gray-900">
              Running {activeTab} to {targetValue}…
            </div>
          )}
          {runDiagnostics.data && (
            <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md bg-gray-950 p-3 font-mono text-xs text-green-400 dark:bg-gray-900">
              {runDiagnostics.data.output || '(no output)'}
            </pre>
          )}
          {runDiagnostics.isError && (
            <InlineError>{runDiagnostics.error.message}</InlineError>
          )}
        </form>
      </CardContent>
    </Card>
  );
}
