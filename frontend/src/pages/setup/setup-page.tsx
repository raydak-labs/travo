import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useCompleteSetup } from '@/hooks/use-system';
import { SETUP_STEPS } from '@/pages/setup/setup-step-constants';
import { StepIndicator, SetupProgressBar } from '@/pages/setup/setup-step-indicator';
import { WelcomeStep } from '@/pages/setup/welcome-step';
import { PasswordStep } from '@/pages/setup/password-step';
import { WifiStep } from '@/pages/setup/wifi-step';
import { APStep } from '@/pages/setup/ap-step';
import { CompleteStep } from '@/pages/setup/complete-step';

export function SetupPage() {
  const [step, setStep] = useState(0);
  const navigate = useNavigate();
  const completeSetup = useCompleteSetup();

  const handleFinish = () => {
    completeSetup.mutate(undefined, {
      onSuccess: () => {
        void navigate({ to: '/dashboard' });
      },
      onError: () => {
        toast.error('Failed to mark setup as complete');
      },
    });
  };

  const total = SETUP_STEPS.length;

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-blue-50 via-white to-blue-100 p-4 dark:from-gray-950 dark:via-gray-900 dark:to-gray-950">
      <Card className="w-full max-w-lg shadow-lg">
        <CardHeader className="pb-2">
          <CardTitle className="text-center text-sm font-medium text-gray-500">
            Initial Setup
          </CardTitle>
        </CardHeader>
        <CardContent>
          <StepIndicator current={step} total={total} />
          <SetupProgressBar step={step} totalSteps={total} />
          {step === 0 && <WelcomeStep onNext={() => setStep(1)} />}
          {step === 1 && <PasswordStep onNext={() => setStep(2)} onBack={() => setStep(0)} />}
          {step === 2 && <WifiStep onNext={() => setStep(3)} onBack={() => setStep(1)} />}
          {step === 3 && <APStep onNext={() => setStep(4)} onBack={() => setStep(2)} />}
          {step === 4 && (
            <CompleteStep onFinish={handleFinish} isPending={completeSetup.isPending} />
          )}
        </CardContent>
      </Card>
    </div>
  );
}
