import { useState } from 'react';
import { Eye, EyeOff, Wifi } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SecurityBadge } from '@/components/wifi/security-badge';

interface WifiConnectDialogProps {
  network: WifiScanResult;
  isConnecting: boolean;
  error: string | null;
  onConnect: (ssid: string, password: string) => void;
  onCancel: () => void;
}

export function WifiConnectDialog({
  network,
  isConnecting,
  error,
  onConnect,
  onCancel,
}: WifiConnectDialogProps) {
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const needsPassword = network.encryption !== 'none';

  function validate(): boolean {
    if (needsPassword && password.length > 0 && password.length < 8) {
      setValidationError('Password must be at least 8 characters');
      return false;
    }
    setValidationError(null);
    return true;
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!validate()) return;
    onConnect(network.ssid, password);
  }

  function handlePasswordChange(e: React.ChangeEvent<HTMLInputElement>) {
    setPassword(e.target.value);
    if (validationError) {
      setValidationError(null);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      role="dialog"
      aria-label="Connect to network"
    >
      <div className="mx-4 w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-900">
        <div className="mb-4 flex items-center gap-3">
          <Wifi className="h-5 w-5 text-blue-500" />
          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{network.ssid}</h2>
            <SecurityBadge encryption={network.encryption} />
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {needsPassword && (
            <div className="relative">
              <Input
                id="wifi-password"
                label="Password"
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={handlePasswordChange}
                placeholder="Enter network password"
                autoFocus
                aria-required="true"
                aria-invalid={validationError ? 'true' : undefined}
                aria-describedby={validationError ? 'password-error' : undefined}
              />
              <button
                type="button"
                className="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                onClick={() => setShowPassword(!showPassword)}
                aria-label={showPassword ? 'Hide password' : 'Show password'}
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
              {validationError && (
                <p
                  id="password-error"
                  className="mt-1 text-sm text-red-600 dark:text-red-400"
                  role="alert"
                >
                  {validationError}
                </p>
              )}
            </div>
          )}

          {error && (
            <p className="text-sm text-red-600 dark:text-red-400" role="alert">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onCancel} disabled={isConnecting}>
              Cancel
            </Button>
            <Button type="submit" disabled={isConnecting || (needsPassword && !password)}>
              {isConnecting ? 'Connecting...' : 'Connect'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
