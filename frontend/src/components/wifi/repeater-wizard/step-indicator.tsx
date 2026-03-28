import type { RepeaterWizardStep } from './types';

const STEPS: RepeaterWizardStep[] = ['select-upstream', 'configure-ap', 'review'];
const LABELS = ['Upstream', 'AP Config', 'Review'];

export function RepeaterWizardStepIndicator({ step }: { step: RepeaterWizardStep }) {
  const stepIndex = STEPS.indexOf(step);

  return (
    <div className="flex items-center justify-center gap-2 py-2">
      {STEPS.map((s, i) => {
        const isActive = s === step;
        const isPast = i < stepIndex;
        return (
          <div key={s} className="flex items-center gap-2">
            {i > 0 && (
              <div
                className={`h-px w-6 ${isPast || isActive ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-700'}`}
              />
            )}
            <div className="flex flex-col items-center gap-1">
              <div
                className={`flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium ${
                  isActive
                    ? 'bg-blue-600 text-white'
                    : isPast
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
                      : 'bg-gray-200 text-gray-500 dark:bg-gray-800 dark:text-gray-400'
                }`}
              >
                {i + 1}
              </div>
              <span className="text-xs text-gray-500">{LABELS[i]}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
