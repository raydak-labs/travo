import { Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { SSHKey } from '@shared/index';

type SSHKeysListProps = {
  keys: SSHKey[];
  deletePending: boolean;
  onDelete: (index: number) => void;
};

export function SSHKeysList({ keys, deletePending, onDelete }: SSHKeysListProps) {
  if (keys.length === 0) {
    return <p className="text-sm text-muted-foreground">No keys configured</p>;
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
              {k.comment && <span className="truncate text-sm text-foreground">{k.comment}</span>}
            </div>
            <p className="mt-1 truncate font-mono text-xs text-muted-foreground">
              {k.key.slice(0, 40)}…
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="shrink-0 text-destructive hover:text-destructive"
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
