import { LOG_LEVEL_COLORS } from './logs-constants';

/** Dense uppercase chips for log terminal; ui/Badge (rounded-full, pastel) not visual parity. */
export function LogsLevelBadge({ level }: { level: string }) {
  if (!level) return null;
  const color =
    LOG_LEVEL_COLORS[level] ?? 'bg-gray-300 text-gray-800 dark:bg-gray-600 dark:text-gray-100';
  return (
    <span
      className={`mr-2 inline-block rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase leading-none ${color}`}
    >
      {level}
    </span>
  );
}
