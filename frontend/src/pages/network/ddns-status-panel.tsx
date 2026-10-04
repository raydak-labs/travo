import { statusDotClass, statusDotIdleClass } from '@/lib/status-dot';
import type { DDNSStatus } from '@shared/index';

type DdnsStatusPanelProps = {
  status: DDNSStatus | undefined;
};

export function DdnsStatusPanel({ status }: DdnsStatusPanelProps) {
  if (!status || (!status.running && !status.public_ip)) return null;

  return (
    <div className="flex items-center gap-3">
      <span
        aria-hidden="true"
        className={`inline-block h-2.5 w-2.5 rounded-full ${
          status.running ? statusDotClass(true) : statusDotIdleClass
        }`}
      />
      <span className="sr-only">Dynamic DNS {status.running ? 'running' : 'stopped'}</span>
      <div className="flex-1 text-sm">
        <span className="font-medium text-gray-900 dark:text-white">
          {status.running ? 'Running' : 'Stopped'}
        </span>
        {status.public_ip ? (
          <span className="ml-2 text-gray-500 dark:text-gray-400">IP: {status.public_ip}</span>
        ) : null}
        {status.last_update ? (
          <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
            Updated: {status.last_update}
          </span>
        ) : null}
      </div>
    </div>
  );
}
