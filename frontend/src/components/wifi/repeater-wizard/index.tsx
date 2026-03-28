import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import type { RepeaterWizardProps } from './types';
import { useRepeaterWizard } from './use-repeater-wizard';
import { RepeaterWizardStepIndicator } from './step-indicator';
import { RepeaterWizardSelectUpstreamStep } from './select-upstream-step';
import { RepeaterWizardConfigureApStep } from './configure-ap-step';
import { RepeaterWizardReviewStep } from './review-step';
import { RepeaterWizardDoneStep } from './done-step';

export type { RepeaterWizardProps } from './types';

export function RepeaterWizard({ open, onOpenChange }: RepeaterWizardProps) {
  const w = useRepeaterWizard(open);

  function handleClose(isOpen: boolean) {
    if (!isOpen) {
      w.reset();
    }
    onOpenChange(isOpen);
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Repeater Setup Wizard</DialogTitle>
          <DialogDescription>Set up your router as a WiFi repeater in 3 steps.</DialogDescription>
        </DialogHeader>

        <RepeaterWizardStepIndicator step={w.step} />

        {w.step === 'select-upstream' && (
          <RepeaterWizardSelectUpstreamStep
            selectedNetwork={w.selectedNetwork}
            upstream={w.upstream}
            setUpstream={w.setUpstream}
            needsPassword={w.needsPassword}
            canProceedUpstream={w.canProceedUpstream}
            scanResults={w.scanResults}
            scanLoading={w.scanLoading}
            onRefreshScan={() => void w.refetch()}
            onSelectNetwork={w.handleSelectNetwork}
            onClearSelection={() => w.setSelectedNetwork(null)}
            onNext={() => w.setStep('configure-ap')}
            onCancel={() => handleClose(false)}
          />
        )}

        {w.step === 'configure-ap' && (
          <RepeaterWizardConfigureApStep
            upstream={w.upstream}
            apConfig={w.apConfig}
            setApConfig={w.setApConfig}
            canProceedAP={w.canProceedAP}
            onBack={() => w.setStep('select-upstream')}
            onNext={() => w.setStep('review')}
          />
        )}

        {w.step === 'review' && !w.done && (
          <RepeaterWizardReviewStep
            upstream={w.upstream}
            selectedNetwork={w.selectedNetwork}
            effectiveAPSSID={w.effectiveAPSSID}
            effectiveAPEncryption={w.effectiveAPEncryption}
            apConfig={w.apConfig}
            applyError={w.applyError}
            applying={w.applying}
            onBack={() => w.setStep('configure-ap')}
            onApply={() => void w.handleApply()}
          />
        )}

        {w.done && (
          <RepeaterWizardDoneStep
            upstreamSsid={w.upstream.ssid}
            effectiveAPSSID={w.effectiveAPSSID}
            onDone={() => handleClose(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
