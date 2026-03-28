import { useEffect, useState } from 'react';
import { Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { useNTPConfig, useSetNTPConfig, useSyncNTP } from '@/hooks/use-system';

export function NTPConfigCard() {
  const { data: ntpConfig, isLoading: ntpLoading } = useNTPConfig();
  const setNTPMutation = useSetNTPConfig();
  const syncNTPMutation = useSyncNTP();

  const [isEditing, setIsEditing] = useState(false);
  const [ntpEnabled, setNtpEnabled] = useState(true);
  const [ntpServers, setNtpServers] = useState<string[]>([]);
  const [ntpNewServer, setNtpNewServer] = useState('');

  useEffect(() => {
    if (ntpConfig) {
      setNtpEnabled(ntpConfig.enabled);
      setNtpServers([...ntpConfig.servers]);
    }
  }, [ntpConfig]);

  const handleCancel = () => {
    if (ntpConfig) {
      setNtpEnabled(ntpConfig.enabled);
      setNtpServers([...ntpConfig.servers]);
    }
    setNtpNewServer('');
    setIsEditing(false);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">NTP Configuration</CardTitle>
        <Clock className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {ntpLoading ? (
          <Skeleton className="h-4 w-1/2" />
        ) : (
          <div className="space-y-4">
            {!isEditing ? (
              <div className="space-y-3">
                <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
                  <div className="flex items-center justify-between">
                    <span className="text-gray-500">NTP</span>
                    <span className="text-gray-900 dark:text-white">
                      {ntpEnabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>

                  <div className="mt-2">
                    <div className="text-xs text-gray-500">Servers</div>
                    <div className="mt-1 font-mono text-sm text-gray-900 dark:text-white">
                      {ntpServers.length > 0 ? ntpServers.join(', ') : '—'}
                    </div>
                  </div>
                </div>

                <div className="flex gap-2 flex-wrap">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => syncNTPMutation.mutate()}
                    disabled={syncNTPMutation.isPending}
                    title="Force a one-shot NTP sync with pool.ntp.org"
                  >
                    {syncNTPMutation.isPending ? 'Syncing…' : 'Sync Now'}
                  </Button>

                  <Button
                    size="sm"
                    onClick={() => setIsEditing(true)}
                    disabled={setNTPMutation.isPending}
                    title="Edit NTP enablement and server list"
                  >
                    Edit NTP Settings
                  </Button>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <label htmlFor="ntp-enabled" className="text-sm text-gray-700 dark:text-gray-300">
                    Enable NTP time synchronization
                  </label>
                  <Switch
                    id="ntp-enabled"
                    label="Enable NTP"
                    checked={ntpEnabled}
                    onChange={(e) => setNtpEnabled(e.target.checked)}
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-xs text-gray-500">NTP Servers</label>
                  {ntpServers.map((server, idx) => (
                    <div key={idx} className="flex items-center gap-2">
                      <Input
                        className="h-8 text-sm"
                        value={server}
                        onChange={(e) => {
                          const updated = [...ntpServers];
                          updated[idx] = e.target.value;
                          setNtpServers(updated);
                        }}
                      />
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-8 px-2 text-red-500 hover:text-red-700"
                        onClick={() => setNtpServers(ntpServers.filter((_, i) => i !== idx))}
                      >
                        Remove
                      </Button>
                    </div>
                  ))}

                  <div className="flex items-center gap-2">
                    <Input
                      className="h-8 text-sm"
                      placeholder="Add NTP server address"
                      value={ntpNewServer}
                      onChange={(e) => setNtpNewServer(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && ntpNewServer.trim()) {
                          e.preventDefault();
                          setNtpServers([...ntpServers, ntpNewServer.trim()]);
                          setNtpNewServer('');
                        }
                      }}
                    />
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="h-8"
                      disabled={!ntpNewServer.trim()}
                      onClick={() => {
                        setNtpServers([...ntpServers, ntpNewServer.trim()]);
                        setNtpNewServer('');
                      }}
                    >
                      Add
                    </Button>
                  </div>
                </div>

                <div className="flex gap-2 flex-wrap">
                  <Button
                    onClick={() =>
                      setNTPMutation.mutate(
                        { enabled: ntpEnabled, servers: ntpServers },
                        {
                          onSuccess: () => {
                            setIsEditing(false);
                            setNtpNewServer('');
                          },
                        },
                      )
                    }
                    disabled={setNTPMutation.isPending}
                    size="sm"
                  >
                    {setNTPMutation.isPending ? 'Saving…' : 'Save NTP Settings'}
                  </Button>

                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => syncNTPMutation.mutate()}
                    disabled={syncNTPMutation.isPending}
                    title="Force a one-shot NTP sync with pool.ntp.org"
                  >
                    {syncNTPMutation.isPending ? 'Syncing…' : 'Sync Now'}
                  </Button>

                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleCancel}
                    disabled={setNTPMutation.isPending}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
