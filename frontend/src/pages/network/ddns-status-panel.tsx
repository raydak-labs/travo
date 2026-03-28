import type { DDNSStatus } from '@shared/index';

type DdnsStatusPanelProps = {
  status: DDNSStatus | undefined;
};

export function DdnsStatusPanel({ status }: DdnsStatusPanelProps) {
  if (!status || (!status.running && !status.public_ip)) return null;

  return (
    <div className="flex items-center gap-3 rounded-md bg-gray-50 p-3 dark:bg-gray-900">
      <span
        className={`inline-block h-2.5 w-2.5 rounded-full ${
          status.running
            ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.6)]'
            : 'bg-gray-300 dark:bg-gray-600'
        }`}
      />
      <div className="flex-1 text-sm">
        <span className="font-medium text-gray-900 dark:text-white">
          {status.running ? 'Running' : 'Stopped'}
        </span>
        {status.public_ip ? (
          <span className="ml-2 text-gray-500">IP: {status.public_ip}</span>
        ) : null}
        {status.last_update ? (
          <span className="ml-2 text-xs text-gray-400">Updated: {status.last_update}</span>
        ) : null}
      </div>
    </div>
  );
}
