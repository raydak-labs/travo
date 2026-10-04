import type { SystemInfo } from '@shared/index';
import { statusDotClass } from '@/lib/status-dot';

type HeaderRouterStatusProps = {
  systemInfo: SystemInfo | undefined;
  systemError: boolean;
};

export function HeaderRouterStatus({ systemInfo, systemError }: HeaderRouterStatusProps) {
  const isConnected = !!systemInfo && !systemError;
  const label = isConnected
    ? `Connected to ${systemInfo?.hostname ?? 'router'}`
    : 'Connection lost';

  return (
    <>
      {systemInfo?.hostname && (
        <span className="hidden text-xs text-gray-500 sm:block dark:text-gray-400">
          {systemInfo.hostname}
        </span>
      )}
      {/* The dot was colour-only and carried its state in a `title` on a
          non-interactive span, which is not reliably exposed. A deuteranopic
          user could not tell the states, and a screen reader user had no
          channel at all to learn the router dropped — the single most important
          global status in the app. */}
      <span role="status" aria-live="polite" className="inline-flex items-center">
        <span aria-hidden="true" className={`inline-block h-2 w-2 rounded-full ${statusDotClass(isConnected)}`} />
        <span className="sr-only">{label}</span>
      </span>
    </>
  );
}
