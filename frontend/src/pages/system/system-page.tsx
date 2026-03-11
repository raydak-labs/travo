import { useState, useEffect, useRef } from 'react';
import {
  Server,
  Cpu,
  HardDrive,
  Clock,
  KeyRound,
  Pencil,
  Lightbulb,
  Download,
  Upload,
  AlertTriangle,
  Zap,
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import {
  useSystemInfo,
  useSystemStats,
  useReboot,
  useFactoryReset,
  useChangePassword,
  useSetHostname,
  useLEDStatus,
  useSetLEDStealth,
  useTimezone,
  useSetTimezone,
  useBackup,
  useRestore,
  useFirmwareUpgrade,
  useNTPConfig,
  useSetNTPConfig,
} from '@/hooks/use-system';
import { formatBytes, formatUptime } from '@/lib/utils';

const TIMEZONES = [
  { zonename: 'UTC', timezone: 'UTC0' },
  { zonename: 'Europe/London', timezone: 'GMT0BST,M3.5.0/1,M10.5.0' },
  { zonename: 'Europe/Berlin', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Paris', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Rome', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Madrid', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Moscow', timezone: 'MSK-3' },
  { zonename: 'America/New_York', timezone: 'EST5EDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Chicago', timezone: 'CST6CDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Denver', timezone: 'MST7MDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Los_Angeles', timezone: 'PST8PDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Sao_Paulo', timezone: '<-03>3' },
  { zonename: 'Asia/Tokyo', timezone: 'JST-9' },
  { zonename: 'Asia/Shanghai', timezone: 'CST-8' },
  { zonename: 'Asia/Kolkata', timezone: 'IST-5:30' },
  { zonename: 'Asia/Dubai', timezone: '<+04>-4' },
  { zonename: 'Asia/Singapore', timezone: '<+08>-8' },
  { zonename: 'Asia/Seoul', timezone: 'KST-9' },
  { zonename: 'Australia/Sydney', timezone: 'AEST-10AEDT,M10.1.0,M4.1.0/3' },
  { zonename: 'Pacific/Auckland', timezone: 'NZST-12NZDT,M9.5.0,M4.1.0/3' },
  { zonename: 'Africa/Cairo', timezone: 'EET-2EEST,M4.5.5/0,M10.5.4/24' },
  { zonename: 'Africa/Johannesburg', timezone: 'SAST-2' },
] as const;

export function SystemPage() {
  const { data: info, isLoading: infoLoading, refetch: refetchInfo } = useSystemInfo();
  const { data: stats, isLoading: statsLoading } = useSystemStats();
  const rebootMutation = useReboot();
  const factoryResetMutation = useFactoryReset();
  const changePasswordMutation = useChangePassword();
  const setHostnameMutation = useSetHostname();
  const { data: ledStatus } = useLEDStatus();
  const setLEDStealthMutation = useSetLEDStealth();
  const { data: timezoneConfig, isLoading: tzLoading } = useTimezone();
  const setTz = useSetTimezone();
  const backup = useBackup();
  const restore = useRestore();
  const firmwareUpgrade = useFirmwareUpgrade();
  const { data: ntpConfig, isLoading: ntpLoading } = useNTPConfig();
  const setNTPMutation = useSetNTPConfig();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const firmwareInputRef = useRef<HTMLInputElement>(null);
  const [showRebootConfirm, setShowRebootConfirm] = useState(false);
  const [showFactoryResetDialog, setShowFactoryResetDialog] = useState(false);
  const [showFirmwareDialog, setShowFirmwareDialog] = useState(false);
  const [firmwareFile, setFirmwareFile] = useState<File | null>(null);
  const [keepSettings, setKeepSettings] = useState(true);
  const [editingHostname, setEditingHostname] = useState(false);
  const [hostnameValue, setHostnameValue] = useState('');
  const [selectedTz, setSelectedTz] = useState<string>('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [ntpEnabled, setNtpEnabled] = useState(true);
  const [ntpServers, setNtpServers] = useState<string[]>([]);
  const [ntpNewServer, setNtpNewServer] = useState('');

  useEffect(() => {
    if (timezoneConfig?.zonename) {
      setSelectedTz(timezoneConfig.zonename);
    }
  }, [timezoneConfig]);

  useEffect(() => {
    if (ntpConfig) {
      setNtpEnabled(ntpConfig.enabled);
      setNtpServers([...ntpConfig.servers]);
    }
  }, [ntpConfig]);

  return (
    <div className="space-y-6">
      {/* System Info */}
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
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* Uptime */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Uptime</CardTitle>
          <Clock className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {infoLoading ? (
            <Skeleton className="h-4 w-1/2" />
          ) : info ? (
            <p className="text-lg font-medium text-gray-900 dark:text-white">
              {formatUptime(info.uptime_seconds)}
            </p>
          ) : null}
        </CardContent>
      </Card>

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

      {/* NTP Configuration */}
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
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={ntpEnabled}
                  onChange={(e) => setNtpEnabled(e.target.checked)}
                  className="rounded border-gray-300"
                />
                Enable NTP time synchronization
              </label>
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
              <Button
                onClick={() =>
                  setNTPMutation.mutate({ enabled: ntpEnabled, servers: ntpServers })
                }
                disabled={setNTPMutation.isPending}
                size="sm"
              >
                {setNTPMutation.isPending ? 'Saving…' : 'Save NTP Settings'}
              </Button>
            </div>
          )}
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

      {/* Actions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Actions</CardTitle>
        </CardHeader>
        <CardContent>
          {showRebootConfirm ? (
            <div className="flex items-center gap-3">
              <Badge variant="warning">Confirm reboot?</Badge>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => {
                  rebootMutation.mutate();
                  setShowRebootConfirm(false);
                }}
                disabled={rebootMutation.isPending}
              >
                {rebootMutation.isPending ? 'Rebooting…' : 'Reboot Now'}
              </Button>
              <Button size="sm" variant="outline" onClick={() => setShowRebootConfirm(false)}>
                Cancel
              </Button>
            </div>
          ) : (
            <Button size="sm" variant="destructive" onClick={() => setShowRebootConfirm(true)}>
              Reboot
            </Button>
          )}

          <div className="mt-4 border-t pt-4">
            <Button size="sm" variant="destructive" onClick={() => setShowFactoryResetDialog(true)}>
              Factory Reset
            </Button>
            <p className="mt-1 text-xs text-gray-500">
              Erase all settings and restore factory defaults.
            </p>
          </div>

          <Dialog open={showFactoryResetDialog} onOpenChange={setShowFactoryResetDialog}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2 text-red-600">
                  <AlertTriangle className="h-5 w-5" />
                  Factory Reset
                </DialogTitle>
              </DialogHeader>
              <div className="space-y-3">
                <p className="text-sm text-gray-700 dark:text-gray-300">
                  This will <strong>erase all configuration changes</strong> and restore the device
                  to factory defaults. The device will reboot.
                </p>
                <p className="text-sm text-gray-700 dark:text-gray-300">
                  You will need to reconnect to the default WiFi network after the reset completes.
                </p>
                <div className="rounded-md bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950 dark:text-red-200">
                  <strong>This action cannot be undone.</strong>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setShowFactoryResetDialog(false)}>
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => {
                    factoryResetMutation.mutate();
                    setShowFactoryResetDialog(false);
                  }}
                  disabled={factoryResetMutation.isPending}
                >
                  {factoryResetMutation.isPending ? 'Resetting…' : 'I understand, Factory Reset'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardContent>
      </Card>

      {/* LED Stealth Mode */}
      {ledStatus && ledStatus.led_count > 0 && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">LED Stealth Mode</CardTitle>
            <Lightbulb className="h-4 w-4 text-gray-500" />
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-700 dark:text-gray-300">
                  {ledStatus.stealth_mode
                    ? 'All LEDs are off — stealth mode active'
                    : `${ledStatus.led_count} LED${ledStatus.led_count > 1 ? 's' : ''} active`}
                </p>
              </div>
              <Button
                size="sm"
                variant={ledStatus.stealth_mode ? 'default' : 'outline'}
                disabled={setLEDStealthMutation.isPending}
                onClick={() =>
                  setLEDStealthMutation.mutate({ stealth_mode: !ledStatus.stealth_mode })
                }
              >
                {ledStatus.stealth_mode ? 'Restore LEDs' : 'Go Stealth'}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

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
                    if (
                      window.confirm(
                        'Restore this backup? Current configuration will be overwritten. A reboot will be needed to apply changes.',
                      )
                    ) {
                      restore.mutate(file);
                    }
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

      {/* Change Password */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Firmware Upgrade</CardTitle>
          <Zap className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <p className="text-xs text-gray-500">
              Upload a sysupgrade firmware image (.bin) to flash the device.
            </p>
            <div>
              <input
                type="file"
                ref={firmwareInputRef}
                accept=".bin"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) {
                    setFirmwareFile(file);
                  }
                  e.target.value = '';
                }}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => firmwareInputRef.current?.click()}
              >
                <Upload className="mr-2 h-4 w-4" />
                Select Firmware Image
              </Button>
              {firmwareFile && (
                <p className="mt-1 text-xs text-gray-700 dark:text-gray-300">
                  Selected: {firmwareFile.name} ({formatBytes(firmwareFile.size)})
                </p>
              )}
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={keepSettings}
                onChange={(e) => setKeepSettings(e.target.checked)}
                className="rounded border-gray-300"
              />
              Keep current settings
            </label>
            <Button
              variant="destructive"
              size="sm"
              disabled={!firmwareFile || firmwareUpgrade.isPending}
              onClick={() => setShowFirmwareDialog(true)}
            >
              <Zap className="mr-2 h-4 w-4" />
              {firmwareUpgrade.isPending ? 'Flashing…' : 'Upload & Flash'}
            </Button>
          </div>

          <Dialog open={showFirmwareDialog} onOpenChange={setShowFirmwareDialog}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2 text-red-600">
                  <AlertTriangle className="h-5 w-5" />
                  Firmware Upgrade
                </DialogTitle>
              </DialogHeader>
              <div className="space-y-3">
                <p className="text-sm text-gray-700 dark:text-gray-300">
                  You are about to flash <strong>{firmwareFile?.name}</strong> onto the device.
                  {keepSettings
                    ? ' Current settings will be preserved.'
                    : ' All settings will be erased.'}
                </p>
                <div className="rounded-md bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950 dark:text-red-200">
                  <strong>Do not power off the device during the upgrade.</strong> The device will
                  reboot automatically.
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setShowFirmwareDialog(false)}>
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => {
                    if (firmwareFile) {
                      firmwareUpgrade.mutate(
                        { file: firmwareFile, keepSettings },
                        {
                          onSuccess: () => {
                            setFirmwareFile(null);
                          },
                        },
                      );
                    }
                    setShowFirmwareDialog(false);
                  }}
                  disabled={firmwareUpgrade.isPending}
                >
                  {firmwareUpgrade.isPending ? 'Flashing…' : 'Flash Firmware'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardContent>
      </Card>

      {/* Change Password */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Change Password</CardTitle>
          <KeyRound className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (newPassword !== confirmPassword) return;
              changePasswordMutation.mutate(
                { current_password: currentPassword, new_password: newPassword },
                {
                  onSuccess: () => {
                    setCurrentPassword('');
                    setNewPassword('');
                    setConfirmPassword('');
                  },
                },
              );
            }}
          >
            <Input
              type="password"
              placeholder="Current password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
            />
            <Input
              type="password"
              placeholder="New password (min 6 characters)"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              minLength={6}
              required
            />
            <Input
              type="password"
              placeholder="Confirm new password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              minLength={6}
              required
            />
            {newPassword && confirmPassword && newPassword !== confirmPassword && (
              <p className="text-sm text-red-500">Passwords do not match</p>
            )}
            <Button
              type="submit"
              size="sm"
              disabled={
                changePasswordMutation.isPending ||
                !currentPassword ||
                !newPassword ||
                newPassword !== confirmPassword ||
                newPassword.length < 6
              }
            >
              {changePasswordMutation.isPending ? 'Changing…' : 'Change Password'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
