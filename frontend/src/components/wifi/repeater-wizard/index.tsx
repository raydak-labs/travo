import { useRef, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
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
  const confirmRef = useRef<HTMLDivElement>(null);

  // Closing used to reset unconditionally, so Escape, the overlay and the X
  // each threw away the upstream password and AP credentials with no prompt.
  const hasTypedInput =
    w.upstream.password !== '' ||
    w.apConfig.ssid !== '' ||
    w.apConfig.key !== '' ||
    w.step !== 'select-upstream';

  const [confirmingClose, setConfirmingClose] = useState(false);

  function discardAndClose() {
    setConfirmingClose(false);
    w.reset();
    onOpenChange(false);
  }

  function requestClose() {
    if (hasTypedInput) {
      setConfirmingClose(true);
      return;
    }
    discardAndClose();
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(true) : requestClose())}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Repeater Setup Wizard</DialogTitle>
          <DialogDescription>Set up your router as a WiFi repeater in 3 steps.</DialogDescription>
        </DialogHeader>

        <RepeaterWizardStepIndicator step={w.step} onStepClick={w.setStep} />

        {/* Announce step changes: swapping the panel silently leaves a screen
            reader user with no idea where they are, and focus can be left on a
            button that no longer exists. */}
        <p aria-live="polite" className="sr-only">
          {w.done
            ? 'Repeater setup complete'
            : `Step ${STEP_NUMBER[w.step]} of 3: ${STEP_LABEL[w.step]}`}
        </p>
        <h3 tabIndex={-1} ref={confirmRef} className="sr-only">
          {w.done ? 'Repeater setup complete' : STEP_LABEL[w.step]}
        </h3>

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
            onCancel={requestClose}
          />
        )}

        {w.step === 'configure-ap' && (
          <RepeaterWizardConfigureApStep
            upstream={w.upstream}
            apConfig={w.apConfig}
            setApConfig={w.setApConfig}
            apConfigs={w.apConfigs}
            allowApOnStaRadio={w.allowApOnStaRadio}
            setAllowApOnStaRadio={w.setAllowApOnStaRadio}
            canProceedAP={w.canProceedAP}
            onBack={() => w.setStep('select-upstream')}
            onNext={() => w.setStep('review')}
          />
        )}

        {w.step === 'review' && !w.done && (
          <RepeaterWizardReviewStep
            upstream={w.upstream}
            selectedNetwork={w.selectedNetwork}
            apSummaryLine={w.apSummaryLine}
            effectiveAPEncryption={w.effectiveAPEncryption}
            apConfig={w.apConfig}
            allowApOnStaRadio={w.allowApOnStaRadio}
            applyError={w.applyError}
            failedStep={w.failedStep}
            applying={w.applying}
            onBack={() => w.setStep('configure-ap')}
            onApply={() => void w.handleApply()}
          />
        )}

        {w.done && (
          <RepeaterWizardDoneStep
            upstreamSsid={w.upstream.ssid}
            apSummaryLine={w.apSummaryLine}
            onDone={discardAndClose}
          />
        )}
      </DialogContent>

      <ConfirmDialog
        open={confirmingClose}
        onOpenChange={setConfirmingClose}
        title="Discard repeater setup?"
        description="The upstream password and access point details you entered are not saved."
        confirmLabel="Discard"
        onConfirm={discardAndClose}
      />
    </Dialog>
  );
}

const STEP_NUMBER = { 'select-upstream': 1, 'configure-ap': 2, review: 3 } as const;
const STEP_LABEL = {
  'select-upstream': 'Choose upstream network',
  'configure-ap': 'Configure access point',
  review: 'Review and apply',
} as const;
