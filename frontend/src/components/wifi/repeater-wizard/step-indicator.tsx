import type { RepeaterWizardStep } from './types';

const STEPS: RepeaterWizardStep[] = ['select-upstream', 'configure-ap', 'review'];
const LABELS = ['Upstream', 'AP Config', 'Review'];

export function RepeaterWizardStepIndicator({
  step,
  onStepClick,
}: {
  step: RepeaterWizardStep;
  /** Completed steps become clickable; without it you can only go back one at a time. */
  onStepClick?: (step: RepeaterWizardStep) => void;
}) {
  const stepIndex = STEPS.indexOf(step);

  return (
    <ol className="flex items-center justify-center gap-2 py-2">
      {STEPS.map((s, i) => {
        const isActive = s === step;
        const isPast = i < stepIndex;
        // State is not colour-only: the current step carries aria-current and
        // every step is announced as "Step N of 3: Label".
        const description = `Step ${i + 1} of ${STEPS.length}: ${LABELS[i]}${
          isActive ? ', current step' : isPast ? ', completed' : ''
        }`;
        const circle = (
          <span
            aria-hidden="true"
            className={`flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium ${
              isActive
                ? 'bg-blue-600 text-white'
                : isPast
                  ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
                  : 'bg-gray-200 text-gray-500 dark:bg-gray-800 dark:text-gray-400'
            }`}
          >
            {i + 1}
          </span>
        );
        return (
          <li key={s} className="flex items-center gap-2">
            {i > 0 && (
              <span
                aria-hidden="true"
                className={`h-px w-6 ${
                  isPast || isActive ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-700'
                }`}
              />
            )}
            {isPast && onStepClick ? (
              <button
                type="button"
                onClick={() => onStepClick(s)}
                aria-label={`${description}. Go back to this step`}
                className="flex flex-col items-center gap-1 rounded-md px-1 py-0.5 hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:outline-none dark:hover:bg-gray-800"
              >
                {circle}
                <span aria-hidden="true" className="text-xs text-gray-500 dark:text-gray-400">
                  {LABELS[i]}
                </span>
              </button>
            ) : (
              <span
                aria-current={isActive ? 'step' : undefined}
                className="flex flex-col items-center gap-1"
              >
                {circle}
                <span aria-hidden="true" className="text-xs text-gray-500 dark:text-gray-400">
                  {LABELS[i]}
                </span>
                <span className="sr-only">{description}</span>
              </span>
            )}
          </li>
        );
      })}
    </ol>
  );
}
