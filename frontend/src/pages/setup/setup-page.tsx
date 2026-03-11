import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  Wifi,
  Shield,
  KeyRound,
  CheckCircle2,
  Rocket,
  Loader2,
  Eye,
  EyeOff,
  RefreshCw,
  Signal,
  Lock,
} from 'lucide-react';
import { toast } from 'sonner';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useChangePassword } from '@/hooks/use-system';
import { useCompleteSetup } from '@/hooks/use-system';
import { useWifiScan, useWifiConnect, useAPConfigs, useSetAPConfig } from '@/hooks/use-wifi';

const STEPS = ['Welcome', 'Password', 'WiFi', 'Access Point', 'Complete'] as const;

function StepIndicator({ current, total }: { current: number; total: number }) {
  return (
    <div className="mb-8">
      <div className="flex items-center justify-between">
        {Array.from({ length: total }, (_, i) => (
          <div key={i} className="flex flex-1 items-center">
            <div
              className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium ${
                i < current
                  ? 'bg-blue-600 text-white'
                  : i === current
                    ? 'border-2 border-blue-600 bg-blue-50 text-blue-600 dark:bg-blue-950'
                    : 'border-2 border-gray-300 text-gray-400 dark:border-gray-600'
              }`}
            >
              {i < current ? <CheckCircle2 className="h-5 w-5" /> : i + 1}
            </div>
            {i < total - 1 && (
              <div
                className={`mx-2 h-0.5 flex-1 ${
                  i < current ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'
                }`}
              />
            )}
          </div>
        ))}
      </div>
      <div className="mt-2 flex justify-between">
        {STEPS.map((label, i) => (
          <span
            key={label}
            className={`text-xs ${i <= current ? 'text-blue-600 font-medium' : 'text-gray-400'}`}
            style={{ width: `${100 / total}%`, textAlign: 'center' }}
          >
            {label}
          </span>
        ))}
      </div>
    </div>
  );
}

function WelcomeStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="space-y-6 text-center">
      <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg">
        <Wifi className="h-10 w-10 text-white" />
      </div>
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
          Welcome to your Travel Router
        </h2>
        <p className="mt-3 text-gray-600 dark:text-gray-400">
          This setup wizard will help you configure your OpenWrt travel router in a few easy steps.
          You'll set a secure password, connect to an upstream WiFi network, and configure your own
          access point.
        </p>
      </div>
      <div className="grid grid-cols-3 gap-4 pt-4">
        <div className="rounded-lg border p-3 dark:border-gray-700">
          <KeyRound className="mx-auto h-6 w-6 text-blue-500" />
          <p className="mt-2 text-xs text-gray-500">Secure Password</p>
        </div>
        <div className="rounded-lg border p-3 dark:border-gray-700">
          <Wifi className="mx-auto h-6 w-6 text-blue-500" />
          <p className="mt-2 text-xs text-gray-500">WiFi Connection</p>
        </div>
        <div className="rounded-lg border p-3 dark:border-gray-700">
          <Shield className="mx-auto h-6 w-6 text-blue-500" />
          <p className="mt-2 text-xs text-gray-500">AP Configuration</p>
        </div>
      </div>
      <Button onClick={onNext} size="lg" className="mt-4 w-full">
        Get Started <Rocket className="ml-2 h-4 w-4" />
      </Button>
    </div>
  );
}

function PasswordStep({ onNext, onBack }: { onNext: () => void; onBack: () => void }) {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const changePasswordMutation = useChangePassword();

  const isValid = currentPassword && newPassword.length >= 8 && newPassword === confirmPassword;

  const handleSubmit = () => {
    if (!isValid) return;
    changePasswordMutation.mutate(
      { current_password: currentPassword, new_password: newPassword },
      {
        onSuccess: () => {
          toast.success('Password changed successfully');
          onNext();
        },
      },
    );
  };

  return (
    <div className="space-y-6">
      <div className="text-center">
        <KeyRound className="mx-auto h-10 w-10 text-blue-500" />
        <h2 className="mt-3 text-xl font-bold text-gray-900 dark:text-white">
          Change Default Password
        </h2>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
          For security, change the default admin password before using the router.
        </p>
      </div>

      <div className="space-y-4">
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            Current Password
          </label>
          <Input
            type={showPassword ? 'text' : 'password'}
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            placeholder="Enter current password"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            New Password
          </label>
          <div className="relative">
            <Input
              type={showPassword ? 'text' : 'password'}
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="Enter new password (min 8 chars)"
            />
            <button
              type="button"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
              onClick={() => setShowPassword(!showPassword)}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
          {newPassword && newPassword.length < 8 && (
            <p className="mt-1 text-xs text-red-500">Password must be at least 8 characters</p>
          )}
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            Confirm New Password
          </label>
          <Input
            type={showPassword ? 'text' : 'password'}
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="Confirm new password"
          />
          {confirmPassword && newPassword !== confirmPassword && (
            <p className="mt-1 text-xs text-red-500">Passwords do not match</p>
          )}
        </div>
      </div>

      <div className="flex gap-3">
        <Button variant="outline" onClick={onBack} className="flex-1">
          Back
        </Button>
        <Button
          onClick={handleSubmit}
          disabled={!isValid || changePasswordMutation.isPending}
          className="flex-1"
        >
          {changePasswordMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          Change Password
        </Button>
      </div>
      <button
        type="button"
        onClick={onNext}
        className="block w-full text-center text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
      >
        Skip for now
      </button>
    </div>
  );
}

function signalStrength(dbm: number) {
  if (dbm >= -50) return 4;
  if (dbm >= -60) return 3;
  if (dbm >= -70) return 2;
  return 1;
}

function WifiStep({ onNext, onBack }: { onNext: () => void; onBack: () => void }) {
  const { data: networks, isLoading: scanning, refetch: rescan } = useWifiScan();
  const connectMutation = useWifiConnect();
  const [selectedSSID, setSelectedSSID] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  const selectedNetwork = networks?.find((n) => n.ssid === selectedSSID);

  const handleConnect = () => {
    if (!selectedSSID) return;
    connectMutation.mutate(
      {
        ssid: selectedSSID,
        password,
        encryption: selectedNetwork?.encryption,
      },
      { onSuccess: () => onNext() },
    );
  };

  return (
    <div className="space-y-6">
      <div className="text-center">
        <Wifi className="mx-auto h-10 w-10 text-blue-500" />
        <h2 className="mt-3 text-xl font-bold text-gray-900 dark:text-white">Connect to WiFi</h2>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
          Select the upstream WiFi network you want your router to connect to (hotel, cafe, etc.).
        </p>
      </div>

      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
          Available Networks
        </span>
        <Button variant="outline" size="sm" onClick={() => rescan()} disabled={scanning}>
          <RefreshCw className={`mr-1 h-3 w-3 ${scanning ? 'animate-spin' : ''}`} />
          Scan
        </Button>
      </div>

      <div className="max-h-64 space-y-2 overflow-y-auto rounded-lg border p-2 dark:border-gray-700">
        {scanning ? (
          Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-12 w-full" />)
        ) : networks && networks.length > 0 ? (
          networks
            .filter((n) => n.ssid)
            .map((network) => (
              <button
                key={`${network.ssid}-${network.bssid}`}
                className={`flex w-full items-center justify-between rounded-lg p-3 text-left transition-colors ${
                  selectedSSID === network.ssid
                    ? 'bg-blue-50 ring-2 ring-blue-500 dark:bg-blue-950'
                    : 'hover:bg-gray-50 dark:hover:bg-gray-800'
                }`}
                onClick={() => {
                  setSelectedSSID(network.ssid);
                  setPassword('');
                }}
              >
                <div className="flex items-center gap-3">
                  <Signal className="h-4 w-4 text-gray-500" />
                  <div>
                    <span className="text-sm font-medium text-gray-900 dark:text-white">
                      {network.ssid}
                    </span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-gray-400">{network.signal_dbm} dBm</span>
                      <span className="text-xs text-gray-400">{network.band}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {network.encryption !== 'none' && <Lock className="h-3 w-3 text-gray-400" />}
                  <Badge
                    variant={signalStrength(network.signal_dbm) >= 3 ? 'default' : 'secondary'}
                  >
                    {signalStrength(network.signal_dbm)}/4
                  </Badge>
                </div>
              </button>
            ))
        ) : (
          <p className="p-4 text-center text-sm text-gray-400">No networks found. Try scanning.</p>
        )}
      </div>

      {selectedSSID && selectedNetwork?.encryption !== 'none' && (
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            Password for "{selectedSSID}"
          </label>
          <div className="relative">
            <Input
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter WiFi password"
            />
            <button
              type="button"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
              onClick={() => setShowPassword(!showPassword)}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </div>
      )}

      <div className="flex gap-3">
        <Button variant="outline" onClick={onBack} className="flex-1">
          Back
        </Button>
        <Button
          onClick={handleConnect}
          disabled={
            !selectedSSID ||
            (selectedNetwork?.encryption !== 'none' && !password) ||
            connectMutation.isPending
          }
          className="flex-1"
        >
          {connectMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          Connect
        </Button>
      </div>
      <button
        type="button"
        onClick={onNext}
        className="block w-full text-center text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
      >
        Skip for now
      </button>
    </div>
  );
}

function APStep({ onNext, onBack }: { onNext: () => void; onBack: () => void }) {
  const { data: apConfigs, isLoading } = useAPConfigs();
  const setAPMutation = useSetAPConfig();
  const [ssid, setSSID] = useState('');
  const [apPassword, setAPPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  const firstAP = apConfigs?.[0];
  const section = firstAP?.section ?? '';

  // Initialize with current values when loaded
  const effectiveSSID = ssid || firstAP?.ssid || '';
  const effectivePassword = apPassword || firstAP?.key || '';

  const handleSave = () => {
    if (!firstAP) return;
    setAPMutation.mutate(
      {
        section,
        config: {
          ...firstAP,
          ssid: effectiveSSID,
          key: effectivePassword,
        },
      },
      { onSuccess: () => onNext() },
    );
  };

  return (
    <div className="space-y-6">
      <div className="text-center">
        <Shield className="mx-auto h-10 w-10 text-blue-500" />
        <h2 className="mt-3 text-xl font-bold text-gray-900 dark:text-white">
          Configure Access Point
        </h2>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
          Set up your router's WiFi network. Your devices will connect to this network.
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : (
        <div className="space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              Network Name (SSID)
            </label>
            <Input
              value={effectiveSSID}
              onChange={(e) => setSSID(e.target.value)}
              placeholder="e.g. MyTravelRouter"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              Password
            </label>
            <div className="relative">
              <Input
                type={showPassword ? 'text' : 'password'}
                value={effectivePassword}
                onChange={(e) => setAPPassword(e.target.value)}
                placeholder="Set AP password (min 8 chars)"
              />
              <button
                type="button"
                className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                onClick={() => setShowPassword(!showPassword)}
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
            {effectivePassword && effectivePassword.length < 8 && (
              <p className="mt-1 text-xs text-red-500">Password must be at least 8 characters</p>
            )}
          </div>
        </div>
      )}

      <div className="flex gap-3">
        <Button variant="outline" onClick={onBack} className="flex-1">
          Back
        </Button>
        <Button
          onClick={handleSave}
          disabled={
            !effectiveSSID || effectivePassword.length < 8 || setAPMutation.isPending || isLoading
          }
          className="flex-1"
        >
          {setAPMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          Save AP Config
        </Button>
      </div>
      <button
        type="button"
        onClick={onNext}
        className="block w-full text-center text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
      >
        Skip for now
      </button>
    </div>
  );
}

function CompleteStep({ onFinish, isPending }: { onFinish: () => void; isPending: boolean }) {
  return (
    <div className="space-y-6 text-center">
      <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-green-500 to-green-600 shadow-lg">
        <CheckCircle2 className="h-10 w-10 text-white" />
      </div>
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Setup Complete!</h2>
        <p className="mt-3 text-gray-600 dark:text-gray-400">
          Your travel router is configured and ready to use. You can adjust all settings later from
          the dashboard.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4 text-left text-sm dark:bg-gray-900">
        <h3 className="mb-2 font-medium text-gray-900 dark:text-white">What's next?</h3>
        <ul className="space-y-1 text-gray-600 dark:text-gray-400">
          <li>• Monitor your connection from the Dashboard</li>
          <li>• Set up a VPN for secure browsing</li>
          <li>• Install additional services (AdGuard, etc.)</li>
          <li>• Configure advanced network settings</li>
        </ul>
      </div>
      <Button onClick={onFinish} size="lg" className="w-full" disabled={isPending}>
        {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        Go to Dashboard
      </Button>
    </div>
  );
}

export function SetupPage() {
  const [step, setStep] = useState(0);
  const navigate = useNavigate();
  const completeSetup = useCompleteSetup();

  const handleFinish = () => {
    completeSetup.mutate(undefined, {
      onSuccess: () => {
        void navigate({ to: '/dashboard' });
      },
      onError: () => {
        toast.error('Failed to mark setup as complete');
      },
    });
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-blue-50 via-white to-blue-100 p-4 dark:from-gray-950 dark:via-gray-900 dark:to-gray-950">
      <Card className="w-full max-w-lg shadow-lg">
        <CardHeader className="pb-2">
          <CardTitle className="text-center text-sm font-medium text-gray-500">
            Initial Setup
          </CardTitle>
        </CardHeader>
        <CardContent>
          <StepIndicator current={step} total={STEPS.length} />
          <Progress value={((step + 1) / STEPS.length) * 100} className="mb-6" />
          {step === 0 && <WelcomeStep onNext={() => setStep(1)} />}
          {step === 1 && <PasswordStep onNext={() => setStep(2)} onBack={() => setStep(0)} />}
          {step === 2 && <WifiStep onNext={() => setStep(3)} onBack={() => setStep(1)} />}
          {step === 3 && <APStep onNext={() => setStep(4)} onBack={() => setStep(2)} />}
          {step === 4 && (
            <CompleteStep onFinish={handleFinish} isPending={completeSetup.isPending} />
          )}
        </CardContent>
      </Card>
    </div>
  );
}
