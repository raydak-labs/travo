import type { UseFormRegister } from 'react-hook-form';
import { Radio } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import type { APConfig } from '@shared/index';
import type { APRadioFormValues } from '@/lib/schemas/wifi-forms';

type ApRadioFormHeaderRowProps = {
  ap: APConfig;
  bandLabel: string;
  register: UseFormRegister<APRadioFormValues>;
};

export function ApRadioFormHeaderRow({ ap, bandLabel, register }: ApRadioFormHeaderRowProps) {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2">
        <Radio className="h-4 w-4 text-gray-500" />
        <span className="text-sm font-medium text-gray-900 dark:text-white">{ap.radio}</span>
        <Badge variant="outline">{bandLabel}</Badge>
        <span className="text-xs text-gray-500">Ch {ap.channel}</span>
      </div>
      <Switch id={`ap-enabled-${ap.section}`} label="Enabled" {...register('enabled')} />
    </div>
  );
}
