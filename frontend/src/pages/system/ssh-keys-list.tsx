import { Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/ui/empty-state';
import type { SSHKey } from '@shared/index';

type SSHKeysListProps = {
  keys: SSHKey[];
  deletePending: boolean;
  onDelete: (index: number) => void;
};

export function SSHKeysList({ keys, deletePending, onDelete }: SSHKeysListProps) {
  if (keys.length === 0) {
    return <EmptyState message="No keys configured" />;
  }

  return (
    <ul className="space-y-2">
      {keys.map((k) => (
        <li
          key={k.index}
          className="flex items-center justify-between gap-3 rounded-md border px-3 py-2"
        >
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <Badge variant="secondary" className="shrink-0 font-mono text-xs">
                {k.key.split(' ')[0] ?? 'key'}
              </Badge>
              {k.comment && <span className="truncate text-sm">{k.comment}</span>}
            </div>
            <p className="mt-1 truncate font-mono text-xs text-gray-500 dark:text-gray-400">
              {k.key.slice(0, 40)}…
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="shrink-0 text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300"
            disabled={deletePending}
            onClick={() => onDelete(k.index)}
            aria-label="Delete SSH key"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </li>
      ))}
    </ul>
  );
}
