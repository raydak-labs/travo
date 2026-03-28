import { useState } from 'react';
import { Key, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useSSHKeys, useAddSSHKey, useDeleteSSHKey } from '@/hooks/use-system';

export function SSHKeysCard() {
  const { data: keys = [], isLoading } = useSSHKeys();
  const addSSHKey = useAddSSHKey();
  const deleteSSHKey = useDeleteSSHKey();
  const [newKey, setNewKey] = useState('');

  const isValidKey = newKey.trimStart().startsWith('ssh-') || newKey.trimStart().startsWith('ecdsa-');

  function handleAdd() {
    const trimmed = newKey.trim();
    if (!trimmed) return;
    addSSHKey.mutate({ key: trimmed }, {
      onSuccess: () => setNewKey(''),
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Key className="h-5 w-5" />
          SSH Public Keys
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : keys.length === 0 ? (
          <p className="text-sm text-muted-foreground">No keys configured</p>
        ) : (
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
                    {k.comment && (
                      <span className="truncate text-sm text-foreground">{k.comment}</span>
                    )}
                  </div>
                  <p className="mt-1 truncate font-mono text-xs text-muted-foreground">
                    {k.key.slice(0, 40)}…
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="shrink-0 text-destructive hover:text-destructive"
                  disabled={deleteSSHKey.isPending}
                  onClick={() => deleteSSHKey.mutate(k.index)}
                  aria-label="Delete SSH key"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </li>
            ))}
          </ul>
        )}

        <div className="space-y-2 pt-2">
          <p className="text-sm font-medium">Add a new public key</p>
          <textarea
            className="h-24 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            placeholder="ssh-ed25519 AAAA… user@host"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            spellCheck={false}
          />
          {newKey.trim().length > 0 && !isValidKey && (
            <p className="text-xs text-destructive">
              Key must start with <code>ssh-</code> or <code>ecdsa-</code>
            </p>
          )}
          <Button
            onClick={handleAdd}
            disabled={!isValidKey || addSSHKey.isPending}
            className="w-full sm:w-auto"
          >
            {addSSHKey.isPending ? 'Adding…' : 'Add Key'}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
