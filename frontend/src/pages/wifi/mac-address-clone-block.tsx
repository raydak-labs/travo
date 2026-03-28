import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Shuffle, RotateCcw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { MacCloneFormValues } from '@/lib/schemas/wifi-forms';

export type MacAddressRow = {
  interface: string;
  current_mac?: string;
  custom_mac?: string;
};

type MacAddressCloneBlockProps = {
  mac: MacAddressRow;
  register: UseFormRegister<MacCloneFormValues>;
  errors: FieldErrors<MacCloneFormValues>;
  onRandomLocal: () => void;
  onRandomizeApply: () => void;
  onResetDefault: () => void;
  setMacPending: boolean;
  randomizePending: boolean;
};

export function MacAddressCloneBlock({
  mac,
  register,
  errors,
  onRandomLocal,
  onRandomizeApply,
  onResetDefault,
  setMacPending,
  randomizePending,
}: MacAddressCloneBlockProps) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Badge variant="outline">STA Interface</Badge>
        {mac.current_mac && (
          <span className="text-sm font-mono text-gray-600 dark:text-gray-400">
            Current: {mac.current_mac}
          </span>
        )}
      </div>
      {mac.custom_mac && (
        <p className="text-xs text-amber-600 dark:text-amber-400">
          Custom MAC active: {mac.custom_mac}
        </p>
      )}
      <div className="space-y-2">
        <label
          htmlFor="mac-input"
          className="text-xs font-medium text-gray-600 dark:text-gray-400"
        >
          Custom MAC Address
        </label>
        <Input
          id="mac-input"
          placeholder="AA:BB:CC:DD:EE:FF"
          className="font-mono"
          aria-invalid={errors.custom_mac ? 'true' : undefined}
          aria-describedby={errors.custom_mac ? 'mac-clone-err' : undefined}
          {...register('custom_mac')}
        />
        {errors.custom_mac ? (
          <p id="mac-clone-err" className="text-xs text-red-500" role="alert">
            {errors.custom_mac.message}
          </p>
        ) : null}
      </div>
      <div className="flex flex-wrap gap-2">
        <Button type="button" size="sm" onClick={onRandomLocal}>
          <Shuffle className="mr-1 h-4 w-4" />
          Random
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={randomizePending}
          onClick={onRandomizeApply}
        >
          <Shuffle className="mr-1 h-4 w-4" />
          {randomizePending ? 'Randomizing...' : 'Randomize & Apply'}
        </Button>
        <Button type="submit" size="sm" disabled={setMacPending}>
          {setMacPending ? 'Applying...' : 'Apply'}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={setMacPending}
          onClick={onResetDefault}
        >
          <RotateCcw className="mr-1 h-4 w-4" />
          Reset to Default
        </Button>
      </div>
    </div>
  );
}
