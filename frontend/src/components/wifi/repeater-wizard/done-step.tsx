import { CheckCircle2 } from 'lucide-react';
import { Button } from '@/components/ui/button';

type DoneStepProps = {
  upstreamSsid: string;
  effectiveAPSSID: string;
  onDone: () => void;
};

export function RepeaterWizardDoneStep({
  upstreamSsid,
  effectiveAPSSID,
  onDone,
}: DoneStepProps) {
  return (
    <div className="flex flex-col items-center gap-4 py-6">
      <CheckCircle2 className="h-12 w-12 text-green-500" />
      <div className="text-center">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Repeater Setup Complete</h3>
        <p className="mt-1 text-sm text-gray-500">
          Your router is now repeating <strong>{upstreamSsid}</strong> as{' '}
          <strong>{effectiveAPSSID}</strong>.
        </p>
      </div>
      <Button onClick={onDone}>Done</Button>
    </div>
  );
}
