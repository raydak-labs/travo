import type { SystemInfo } from '@shared/index';

type HeaderRouterStatusProps = {
  systemInfo: SystemInfo | undefined;
  systemError: boolean;
};

export function HeaderRouterStatus({ systemInfo, systemError }: HeaderRouterStatusProps) {
  const isConnected = !!systemInfo && !systemError;

  return (
    <>
      {systemInfo?.hostname && (
        <span className="hidden text-xs text-gray-500 sm:block dark:text-gray-400">
          {systemInfo.hostname}
        </span>
      )}
      <span
        className={`inline-block h-2 w-2 rounded-full ${
          isConnected
            ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.6)]'
            : 'bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.6)]'
        }`}
        title={isConnected ? `Connected to ${systemInfo?.hostname ?? 'router'}` : 'Connection lost'}
      />
    </>
  );
}
