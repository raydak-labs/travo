import { useState } from 'react';
import { Eye, EyeOff, WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useWifiConnect } from '@/hooks/use-wifi';

const ENCRYPTION_OPTIONS = [
  { value: 'psk2', label: 'WPA2 (PSK)' },
  { value: 'sae', label: 'WPA3 (SAE)' },
  { value: 'psk', label: 'WPA (PSK)' },
  { value: 'none', label: 'Open (No Password)' },
] as const;

export function WifiHiddenNetworkDialog() {
  const [open, setOpen] = useState(false);
  const [ssid, setSsid] = useState('');
  const [encryption, setEncryption] = useState('psk2');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const connectMutation = useWifiConnect();

  const needsPassword = encryption !== 'none';

  function resetForm() {
    setSsid('');
    setEncryption('psk2');
    setPassword('');
    setShowPassword(false);
    setValidationError(null);
  }

  function handleOpen() {
    resetForm();
    setOpen(true);
  }

  function validate(): boolean {
    if (ssid.trim() === '') {
      setValidationError('SSID is required');
      return false;
    }
    if (needsPassword && password.length > 0 && password.length < 8) {
      setValidationError('Password must be at least 8 characters');
      return false;
    }
    if (needsPassword && password.length === 0) {
      setValidationError('Password is required for encrypted networks');
      return false;
    }
    setValidationError(null);
    return true;
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!validate()) return;
    connectMutation.mutate(
      { ssid: ssid.trim(), password, encryption, hidden: true },
      {
        onSuccess: () => {
          resetForm();
          setOpen(false);
        },
      },
    );
  }

  return (
    <>
      <Button onClick={handleOpen} size="sm" variant="outline">
        <WifiOff className="mr-1.5 h-3.5 w-3.5" />
        Hidden Network
      </Button>

      <Dialog
        open={open}
        onOpenChange={(v) => {
          setOpen(v);
          if (!v) resetForm();
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Connect to Hidden Network</DialogTitle>
            <DialogDescription>
              Enter the network name and credentials for a hidden WiFi network.
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSubmit} className="space-y-4">
            <Input
              id="hidden-ssid"
              label="Network Name (SSID)"
              type="text"
              value={ssid}
              onChange={(e) => {
                setSsid(e.target.value);
                if (validationError) setValidationError(null);
              }}
              placeholder="Enter network name"
              autoFocus
              aria-required="true"
            />

            <div className="space-y-1.5">
              <label
                htmlFor="hidden-encryption"
                className="text-sm font-medium text-gray-700 dark:text-gray-300"
              >
                Encryption
              </label>
              <Select value={encryption} onValueChange={setEncryption}>
                <SelectTrigger id="hidden-encryption">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ENCRYPTION_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {needsPassword && (
              <div className="relative">
                <Input
                  id="hidden-password"
                  label="Password"
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    if (validationError) setValidationError(null);
                  }}
                  placeholder="Enter network password"
                  aria-required="true"
                  aria-invalid={validationError ? 'true' : undefined}
                  aria-describedby={validationError ? 'hidden-password-error' : undefined}
                />
                <button
                  type="button"
                  className="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  onClick={() => setShowPassword(!showPassword)}
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            )}

            {validationError && (
              <p className="text-sm text-red-600 dark:text-red-400" role="alert">
                {validationError}
              </p>
            )}

            {connectMutation.error && (
              <p className="text-sm text-red-600 dark:text-red-400" role="alert">
                {connectMutation.error.message}
              </p>
            )}

            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setOpen(false)}
                disabled={connectMutation.isPending}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={connectMutation.isPending || ssid.trim() === ''}>
                {connectMutation.isPending ? 'Connecting...' : 'Connect'}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
