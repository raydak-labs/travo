import type { HardwareButton } from '@shared/index';
import { Button } from '@/components/ui/button';
import { hardwareButtonActionLabel } from './hardware-button-actions';

type HardwareButtonsSummaryViewProps = {
  buttons: readonly HardwareButton[];
  onEdit: () => void;
  editDisabled: boolean;
};

export function HardwareButtonsSummaryView({
  buttons,
  onEdit,
  editDisabled,
}: HardwareButtonsSummaryViewProps) {
  return (
    <div className="space-y-3">
      {buttons.map((btn) => (
        <div key={btn.name} className="flex items-center justify-between gap-4">
          <span className="font-mono text-sm capitalize">{btn.name}</span>
          <span className="text-sm text-gray-900 dark:text-white">
            {hardwareButtonActionLabel(btn.action)}
          </span>
        </div>
      ))}

      <Button
        size="sm"
        type="button"
        disabled={editDisabled}
        onClick={onEdit}
        title="Edit button-to-action mapping"
      >
        Edit Button Actions
      </Button>
    </div>
  );
}
