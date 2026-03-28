import { useState, useMemo, useEffect } from 'react';
import { Eye, EyeOff, Wifi } from 'lucide-react';
import type { GroupedScanNetwork, WifiBand } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SecurityBadge } from '@/components/wifi/security-badge';

interface WifiConnectDialogProps {
  group: GroupedScanNetwork;
  isConnecting: boolean;
  error: string | null;
  onConnect: (ssid: string, password: string, band?: string) => void;
  onCancel: () => void;
  /** When true, renders inline without overlay */
  embedded?: boolean;
}

function bandLabel(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4 GHz';
  if (b === '5ghz' || b === '5g') return '5 GHz';
  if (b === '6ghz' || b === '6g') return '6 GHz';
  return band;
}

/** Normalize band to WifiBand for API (2.4ghz, 5ghz, 6ghz) */
function toWifiBand(band: string): WifiBand {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4ghz';
  if (b === '5ghz' || b === '5g') return '5ghz';
  if (b === '6ghz' || b === '6g') return '6ghz';
  return band as WifiBand;
}

function signalQuality(dbm: number): string {
  if (dbm >= -50) return 'Excellent';
  if (dbm >= -60) return 'Strong';
  if (dbm >= -70) return 'Good';
  if (dbm >= -80) return 'Fair';
  return 'Weak';
}

const DOWN_SWITCH_THRESHOLD_DBM = -70;

export function WifiConnectDialog({
  group,
  isConnecting,
  error,
  onConnect,
  onCancel,
  embedded,
}: WifiConnectDialogProps) {
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const needsPassword = group.encryption !== 'none';

  const bandOptions = useMemo(() => {
    const byBand = new Map<string, { dbm: number; ap: (typeof group.aps)[0] }>();
    for (const ap of group.aps) {
      const b = ap.band.toLowerCase();
      const key =
        b.includes('2.4') || b === '2.4g'
          ? '2.4ghz'
          : b.includes('5')
            ? '5ghz'
            : b.includes('6')
              ? '6ghz'
              : b;
      const existing = byBand.get(key);
      if (!existing || ap.signal_dbm > existing.dbm) {
        byBand.set(key, { dbm: ap.signal_dbm, ap });
      }
    }
    return Array.from(byBand.entries()).map(([band, { dbm }]) => ({ band, dbm }));
  }, [group]);

  const defaultBand = useMemo(() => {
    if (bandOptions.length <= 1) return bandOptions[0]?.band ?? null;
    const five = bandOptions.find((b) => b.band === '5ghz');
    const two = bandOptions.find((b) => b.band === '2.4ghz');
    if (five && five.dbm >= DOWN_SWITCH_THRESHOLD_DBM) return '5ghz';
    if (two) return '2.4ghz';
    return bandOptions[0]?.band ?? null;
  }, [bandOptions]);

  const [selectedBand, setSelectedBand] = useState<string | null>(() => defaultBand ?? null);

  useEffect(() => {
    if (defaultBand != null) setSelectedBand(defaultBand);
  }, [defaultBand]);

  const effectiveBand = selectedBand ?? defaultBand;

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
    onConnect(group.ssid, password, effectiveBand ? toWifiBand(effectiveBand) : undefined);
  }

  function handlePasswordChange(e: React.ChangeEvent<HTMLInputElement>) {
    setPassword(e.target.value);
    if (validationError) setValidationError(null);
  }

  const content = (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <Wifi className="h-5 w-5 text-blue-500" />
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{group.ssid}</h2>
          <SecurityBadge encryption={group.encryption} />
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        {bandOptions.length > 1 && (
          <div className="space-y-2">
            <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Band</p>
            <div className="space-y-2">
              {bandOptions.map(({ band, dbm }) => (
                <label
                  key={band}
                  className="flex cursor-pointer items-center gap-2 rounded border p-2 has-[:checked]:border-blue-500 has-[:checked]:bg-blue-50 dark:has-[:checked]:bg-blue-950/30"
                >
                  <input
                    type="radio"
                    name="band"
                    value={band}
                    checked={effectiveBand === band}
                    onChange={() => setSelectedBand(band)}
                    className="h-4 w-4"
                  />
                  <span className="text-sm text-gray-900 dark:text-white">
                    {bandLabel(band)} ({dbm} dBm, {signalQuality(dbm)})
                  </span>
                </label>
              ))}
            </div>
          </div>
        )}

        {needsPassword && (
          <div className="relative">
            <Input
              id="wifi-password"
              label="Password"
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={handlePasswordChange}
              placeholder="Enter network password"
              autoFocus={bandOptions.length <= 1}
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
  );

  if (embedded) return content;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      role="dialog"
      aria-label="Connect to network"
    >
      <div className="mx-4 w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-900">
        {content}
      </div>
    </div>
  );
}
