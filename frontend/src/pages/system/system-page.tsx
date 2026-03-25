import { useState, useEffect, useRef } from 'react';
import {
  Server,
  Cpu,
  HardDrive,
  Clock,
  Pencil,
  Download,
  Upload,
  ExternalLink,
  FileEdit,
} from 'lucide-react';
import { toast } from 'sonner';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { NTPConfigCard } from './ntp-config-card';
import { LEDControlCard } from './led-control-card';
import { FirmwareUpgradeCard } from './firmware-upgrade-card';
import { ChangePasswordCard } from './change-password-card';
import { HardwareButtonsCard } from './hardware-buttons-card';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import {
  useSystemInfo,
  useSystemStats,
  useReboot,
  useShutdown,
  useFactoryReset,
  useSetHostname,
  useTimezone,
  useSetTimezone,
  useBackup,
  useRestore,
} from '@/hooks/use-system';
import { useServices, useAdGuardConfig, useSetAdGuardConfig } from '@/hooks/use-services';
import { formatBytes, formatUptime } from '@/lib/utils';
import { TIMEZONES } from '@/lib/timezones';
import { SSHKeysCard } from './ssh-keys-card';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { AlertThresholdsCard } from './alert-thresholds-card';

export function SystemPage() {
  const { data: info, isLoading: infoLoading, refetch: refetchInfo } = useSystemInfo();
  const { data: stats, isLoading: statsLoading } = useSystemStats();
  const rebootMutation = useReboot();
  const shutdownMutation = useShutdown();
  const factoryResetMutation = useFactoryReset();
  const setHostnameMutation = useSetHostname();
  const { data: timezoneConfig, isLoading: tzLoading } = useTimezone();
  const setTz = useSetTimezone();
  const backup = useBackup();
  const restore = useRestore();
  const { data: services = [] } = useServices();
  const adguardConfigQuery = useAdGuardConfig();
  const setAdGuardConfig = useSetAdGuardConfig();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showRebootDialog, setShowRebootDialog] = useState(false);
  const [showShutdownDialog, setShowShutdownDialog] = useState(false);
  const [showRestoreDialog, setShowRestoreDialog] = useState(false);
  const [pendingRestoreFile, setPendingRestoreFile] = useState<File | null>(null);
  const [showFactoryResetDialog, setShowFactoryResetDialog] = useState(false);
  const [editingHostname, setEditingHostname] = useState(false);
  const [hostnameValue, setHostnameValue] = useState('');
  const [selectedTz, setSelectedTz] = useState<string>('');
  const [configEditorOpen, setConfigEditorOpen] = useState(false);
  const [configContent, setConfigContent] = useState('');

  useEffect(() => {
    if (timezoneConfig?.zonename) {
      setSelectedTz(timezoneConfig.zonename);
    }
  }, [timezoneConfig]);

  const adguardRunning = services.some((s) => s.id === 'adguardhome' && s.state === 'running');
  const adguardInstalled = services.some(
    (s) => s.id === 'adguardhome' && s.state !== 'not_installed',
  );

  const handleOpenConfigEditor = async () => {
    const result = await adguardConfigQuery.refetch();
    if (result.data) {
      setConfigContent(result.data.content);
      setConfigEditorOpen(true);
    } else if (result.error) {
      toast.error(
        result.error instanceof Error
          ? result.error.message
          : 'Failed to load AdGuard configuration',
      );
    }
  };

  const handleSaveConfig = () => {
    setAdGuardConfig.mutate(configContent, {
      onSuccess: () => setConfigEditorOpen(false),
    });
  };

  return (
    <div className="space-y-6">
      {/* ── At a Glance ──────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          At a Glance
        </h2>
        <div className="space-y-4">

      {/* System Info (with Uptime merged in) */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">System Information</CardTitle>
          <Server className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {infoLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : info ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">Hostname</span>
                <span className="flex items-center gap-1 text-gray-900 dark:text-white">
                  {editingHostname ? (
                    <form
                      className="flex items-center gap-1"
                      onSubmit={(e) => {
                        e.preventDefault();
                        if (hostnameValue) {
                          setHostnameMutation.mutate(
                            { hostname: hostnameValue },
                            {
                              onSuccess: () => {
                                setEditingHostname(false);
                                refetchInfo();
                              },
                            },
                          );
                        }
                      }}
                    >
                      <Input
                        className="h-6 w-32 text-xs"
                        value={hostnameValue}
                        onChange={(e) => setHostnameValue(e.target.value)}
                        autoFocus
                      />
                      <Button type="submit" size="sm" variant="ghost" className="h-6 px-1 text-xs">
                        Save
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-6 px-1 text-xs"
                        onClick={() => setEditingHostname(false)}
                      >
                        Cancel
                      </Button>
                    </form>
                  ) : (
                    <>
                      {info.hostname}
                      <button
                        className="ml-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                        onClick={() => {
                          setHostnameValue(info.hostname);
                          setEditingHostname(true);
                        }}
                      >
                        <Pencil className="h-3 w-3" />
                      </button>
                    </>
                  )}
                </span>
                <span className="text-gray-500">Model</span>
                <span className="text-gray-900 dark:text-white">{info.model}</span>
                <span className="text-gray-500">Firmware</span>
                <span className="text-gray-900 dark:text-white">{info.firmware_version}</span>
                <span className="text-gray-500">Kernel</span>
                <span className="text-gray-900 dark:text-white">{info.kernel_version}</span>
                <span className="text-gray-500">Uptime</span>
                <span className="text-gray-900 dark:text-white">
                  {formatUptime(info.uptime_seconds)}
                </span>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* System Stats */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">System Stats</CardTitle>
          <Cpu className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent className="space-y-4">
          {statsLoading ? (
            <div className="space-y-4">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : stats ? (
            <>
              {/* CPU */}
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-gray-700 dark:text-gray-300">CPU</span>
                  <span className="text-gray-900 dark:text-white">
                    {stats.cpu.usage_percent.toFixed(1)}%
                    {stats.cpu.temperature_celsius != null && (
                      <span className="ml-2 text-gray-500">{stats.cpu.temperature_celsius}°C</span>
                    )}
                  </span>
                </div>
                <Progress value={stats.cpu.usage_percent} />
                <p className="mt-0.5 text-xs text-gray-500">
                  Load: {stats.cpu.load_average.map((v) => v.toFixed(2)).join(', ')} ·{' '}
                  {stats.cpu.cores} cores
                </p>
              </div>

              {/* Memory */}
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-gray-700 dark:text-gray-300">Memory</span>
                  <span className="text-gray-900 dark:text-white">
                    {stats.memory.usage_percent.toFixed(1)}% ({formatBytes(stats.memory.used_bytes)}{' '}
                    / {formatBytes(stats.memory.total_bytes)})
                  </span>
                </div>
                <Progress value={stats.memory.usage_percent} />
              </div>

              {/* Storage */}
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-gray-700 dark:text-gray-300">
                    <span className="inline-flex items-center gap-1">
                      <HardDrive className="h-3.5 w-3.5" />
                      Storage
                    </span>
                  </span>
                  <span className="text-gray-900 dark:text-white">
                    {stats.storage.usage_percent.toFixed(1)}% (
                    {formatBytes(stats.storage.used_bytes)} /{' '}
                    {formatBytes(stats.storage.total_bytes)})
                  </span>
                </div>
                <Progress value={stats.storage.usage_percent} />
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>

        </div>
      </div>

      {/* ── Configuration ────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Configuration
        </h2>
        <div className="space-y-4">

      {/* Time & Timezone */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Time & Timezone</CardTitle>
          <Clock className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {tzLoading ? (
            <Skeleton className="h-4 w-1/2" />
          ) : (
            <div className="space-y-4">
              <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
                <div className="grid grid-cols-2 gap-2">
                  <span className="text-gray-500">Timezone</span>
                  <span className="text-gray-900 dark:text-white">
                    {timezoneConfig?.zonename || '—'}
                  </span>
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-xs text-gray-500">Change Timezone</label>
                <Select value={selectedTz} onValueChange={setSelectedTz}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select timezone" />
                  </SelectTrigger>
                  <SelectContent>
                    {TIMEZONES.map((tz) => (
                      <SelectItem key={tz.zonename} value={tz.zonename}>
                        {tz.zonename}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button
                onClick={() => {
                  const tz = TIMEZONES.find((t) => t.zonename === selectedTz);
                  if (tz) setTz.mutate({ zonename: tz.zonename, timezone: tz.timezone });
                }}
                disabled={setTz.isPending || !selectedTz}
                size="sm"
              >
                {setTz.isPending ? 'Saving…' : 'Save Timezone'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <NTPConfigCard />
      <ChangePasswordCard />
      <HardwareButtonsCard />
      <LEDControlCard />
      <AlertThresholdsCard />
      <SSHKeysCard />

        </div>
      </div>

      {/* ── Maintenance ──────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Maintenance
        </h2>
        <div className="space-y-4">

      {/* Backup & Restore */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Backup & Restore</CardTitle>
          <Download className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => backup.mutate()}
              disabled={backup.isPending}
            >
              <Download className="mr-2 h-4 w-4" />
              {backup.isPending ? 'Creating backup…' : 'Download Backup'}
            </Button>
            <div>
              <input
                type="file"
                ref={fileInputRef}
                accept=".tar.gz,.gz"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) {
                    setPendingRestoreFile(file);
                    setShowRestoreDialog(true);
                    e.target.value = '';
                  }
                }}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={restore.isPending}
              >
                <Upload className="mr-2 h-4 w-4" />
                {restore.isPending ? 'Restoring…' : 'Restore from Backup'}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <FirmwareUpgradeCard />

        </div>
      </div>

      {/* ── Danger Zone ──────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-red-500 dark:text-red-400">
          Danger Zone
        </h2>
        <Card className="border-red-200 dark:border-red-900">
          <CardContent className="space-y-4 pt-4">
            <p className="text-xs text-gray-500">
              These actions are irreversible or will cause a service interruption. Proceed with
              caution.
            </p>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="destructive" onClick={() => setShowRebootDialog(true)}>
                Reboot
              </Button>
              <Button size="sm" variant="destructive" onClick={() => setShowShutdownDialog(true)}>
                Shut Down
              </Button>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => setShowFactoryResetDialog(true)}
              >
                Factory Reset
              </Button>
            </div>
            <p className="text-xs text-gray-500">
              Shut Down powers off the device — you will need physical access to turn it back on.
            </p>
          </CardContent>
        </Card>
      </div>

      {/* ── Quick Links ──────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Quick Links
        </h2>
        <Card>
          <CardContent className="pt-4">
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="outline" asChild>
                <a
                  href={`http://${window.location.hostname}:8080/cgi-bin/luci`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                  LuCI Web Interface
                </a>
              </Button>
              {adguardRunning && (
                <Button size="sm" variant="outline" asChild>
                  <a
                    href={`http://${window.location.hostname}:3000`}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                    AdGuard Dashboard
                  </a>
                </Button>
              )}
              {adguardInstalled && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleOpenConfigEditor}
                  disabled={adguardConfigQuery.isFetching}
                >
                  <FileEdit className="mr-1.5 h-3.5 w-3.5" />
                  {adguardConfigQuery.isFetching ? 'Loading…' : 'AdGuard Config'}
                </Button>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Dialogs */}
      <ConfirmDialog
        open={showRebootDialog}
        onOpenChange={setShowRebootDialog}
        title="Reboot System"
        description="The router will reboot and be temporarily unreachable."
        warningText="You will lose your connection for 30–60 seconds while the device restarts."
        confirmLabel="Reboot Now"
        isPending={rebootMutation.isPending}
        onConfirm={() => {
          rebootMutation.mutate();
          setShowRebootDialog(false);
        }}
      />

      <ConfirmDialog
        open={showShutdownDialog}
        onOpenChange={setShowShutdownDialog}
        title="Shut Down"
        description="The router will power off. You will need physical access to turn it back on."
        warningText="This will make the router inaccessible until manually powered on."
        confirmLabel="Shut Down"
        isPending={shutdownMutation.isPending}
        onConfirm={() => {
          shutdownMutation.mutate();
          setShowShutdownDialog(false);
        }}
      />

      <ConfirmDialog
        open={showFactoryResetDialog}
        onOpenChange={setShowFactoryResetDialog}
        title="Factory Reset"
        description="This will erase all configuration and restore factory defaults. The device will reboot. You will need to reconnect to the default WiFi network."
        warningText="This action cannot be undone."
        confirmLabel="I understand, Factory Reset"
        isPending={factoryResetMutation.isPending}
        onConfirm={() => {
          factoryResetMutation.mutate();
          setShowFactoryResetDialog(false);
        }}
      />

      <Dialog open={configEditorOpen} onOpenChange={setConfigEditorOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>AdGuard Home Configuration</DialogTitle>
            <DialogDescription>
              Edit the AdGuardHome.yaml configuration file. The service will be restarted after
              saving.
            </DialogDescription>
          </DialogHeader>
          <textarea
            className="h-96 w-full rounded-md border border-gray-300 bg-white p-3 font-mono text-sm text-gray-900 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            value={configContent}
            onChange={(e) => setConfigContent(e.target.value)}
            spellCheck={false}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfigEditorOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveConfig} disabled={setAdGuardConfig.isPending}>
              {setAdGuardConfig.isPending ? 'Saving…' : 'Save & Restart'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={showRestoreDialog}
        onOpenChange={(open) => {
          setShowRestoreDialog(open);
          if (!open) setPendingRestoreFile(null);
        }}
        title="Restore from Backup"
        description="Current configuration will be overwritten. A reboot will be needed to apply changes."
        warningText="This will replace all your current settings with the backup file."
        confirmLabel="Restore"
        isPending={restore.isPending}
        onConfirm={() => {
          if (pendingRestoreFile) {
            restore.mutate(pendingRestoreFile);
            setShowRestoreDialog(false);
            setPendingRestoreFile(null);
          }
        }}
      />
    </div>
  );
}
