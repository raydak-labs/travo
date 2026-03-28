import type { UseFormRegister, UseFormSetValue } from 'react-hook-form';
import type { LogsFilterFormValues } from '@/lib/schemas/logs-forms';
import type { LogEntry } from '@shared/index';
import type { LogTab } from '@/pages/logs/logs-constants';
import { LogsToolbarLevelFilters } from '@/pages/logs/logs-toolbar-level-filters';
import { LogsToolbarServiceFilters } from '@/pages/logs/logs-toolbar-service-filters';
import { LogsToolbarTabsAndSearch } from '@/pages/logs/logs-toolbar-tabs-and-search';

type LogsToolbarProps = {
  activeTab: LogTab;
  setActiveTab: (t: LogTab) => void;
  register: UseFormRegister<LogsFilterFormValues>;
  setValue: UseFormSetValue<LogsFilterFormValues>;
  serviceFilter: string;
  levelFilter: string;
  showCustomInput: boolean;
  filteredLines: readonly LogEntry[];
  onRefresh: () => void;
  onDownload: () => void;
};

export function LogsToolbar({
  activeTab,
  setActiveTab,
  register,
  setValue,
  serviceFilter,
  levelFilter,
  showCustomInput,
  filteredLines,
  onRefresh,
  onDownload,
}: LogsToolbarProps) {
  return (
    <>
      <LogsToolbarTabsAndSearch
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        register={register}
        filteredLines={filteredLines}
        onRefresh={onRefresh}
        onDownload={onDownload}
      />

      {activeTab === 'system' && (
        <LogsToolbarServiceFilters
          register={register}
          setValue={setValue}
          serviceFilter={serviceFilter}
          showCustomInput={showCustomInput}
        />
      )}

      {activeTab === 'system' && (
        <LogsToolbarLevelFilters setValue={setValue} levelFilter={levelFilter} />
      )}
    </>
  );
}
