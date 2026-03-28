import { CheckCircle2 } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Progress } from '@/components/ui/progress';

export const SETUP_STEPS = ['Welcome', 'Password', 'WiFi', 'Access Point', 'Complete'] as const;

export function StepIndicator({ current, total }: { current: number; total: number }) {
  return (
    <div className="mb-8">
      <div className="flex items-center justify-between">
        {Array.from({ length: total }, (_, i) => (
          <div key={i} className="flex flex-1 items-center">
            <div
              className={cn(
                'flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium',
                i < current && 'bg-blue-600 text-white',
                i === current && 'border-2 border-blue-600 bg-blue-50 text-blue-600 dark:bg-blue-950',
                i > current && 'border-2 border-gray-300 text-gray-400 dark:border-gray-600',
              )}
            >
              {i < current ? <CheckCircle2 className="h-5 w-5" /> : i + 1}
            </div>
            {i < total - 1 && (
              <div
                className={cn(
                  'mx-2 h-0.5 flex-1',
                  i < current ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600',
                )}
              />
            )}
          </div>
        ))}
      </div>
      <div className="mt-2 grid grid-cols-5 gap-1">
        {SETUP_STEPS.map((label, i) => (
          <span
            key={label}
            className={cn(
              'text-center text-xs',
              i <= current ? 'font-medium text-blue-600' : 'text-gray-400',
            )}
          >
            {label}
          </span>
        ))}
      </div>
    </div>
  );
}

export function SetupProgressBar({ step, totalSteps }: { step: number; totalSteps: number }) {
  return <Progress value={((step + 1) / totalSteps) * 100} className="mb-6" />;
}
