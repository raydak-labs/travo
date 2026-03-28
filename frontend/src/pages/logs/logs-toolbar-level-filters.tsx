import type { UseFormSetValue } from 'react-hook-form';
import { Filter } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { LogsFilterFormValues } from '@/lib/schemas/logs-forms';
import { LOG_LEVEL_FILTERS } from '@/pages/logs/logs-constants';

type LogsToolbarLevelFiltersProps = {
  setValue: UseFormSetValue<LogsFilterFormValues>;
  levelFilter: string;
};

export function LogsToolbarLevelFilters({ setValue, levelFilter }: LogsToolbarLevelFiltersProps) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-2">
      <Filter className="h-4 w-4 text-gray-400" />
      <span className="text-xs text-gray-500">Level:</span>
      {LOG_LEVEL_FILTERS.map((lf) => (
        <Button
          key={lf.value}
          variant={levelFilter === lf.value ? 'default' : 'outline'}
          size="sm"
          className="h-7 text-xs"
          onClick={() => setValue('levelFilter', lf.value)}
        >
          {lf.label}
        </Button>
      ))}
    </div>
  );
}
