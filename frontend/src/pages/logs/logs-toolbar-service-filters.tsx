import type { UseFormRegister, UseFormSetValue } from 'react-hook-form';
import { Filter } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { LogsFilterFormValues } from '@/lib/schemas/logs-forms';
import { LOG_SERVICE_FILTERS } from '@/pages/logs/logs-constants';

type LogsToolbarServiceFiltersProps = {
  register: UseFormRegister<LogsFilterFormValues>;
  setValue: UseFormSetValue<LogsFilterFormValues>;
  serviceFilter: string;
  showCustomInput: boolean;
};

export function LogsToolbarServiceFilters({
  register,
  setValue,
  serviceFilter,
  showCustomInput,
}: LogsToolbarServiceFiltersProps) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-2">
      <Filter className="h-4 w-4 text-gray-400" />
      {LOG_SERVICE_FILTERS.map((sf) => (
        <Button
          key={sf.value}
          variant={!showCustomInput && serviceFilter === sf.value ? 'default' : 'outline'}
          size="sm"
          className="h-7 text-xs"
          onClick={() => {
            setValue('serviceFilter', sf.value);
            setValue('showCustomInput', false);
            setValue('customService', '');
          }}
        >
          {sf.label}
        </Button>
      ))}
      <Button
        variant={showCustomInput ? 'default' : 'outline'}
        size="sm"
        className="h-7 text-xs"
        onClick={() => {
          setValue('showCustomInput', true);
          setValue('serviceFilter', '');
        }}
      >
        Custom
      </Button>
      {showCustomInput && (
        <Input
          placeholder="Service name…"
          className="h-7 w-40 text-xs"
          autoFocus
          {...register('customService')}
        />
      )}
    </div>
  );
}
