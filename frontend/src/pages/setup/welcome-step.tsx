import { Wifi, KeyRound, Shield, Rocket } from 'lucide-react';
import { Button } from '@/components/ui/button';

export function WelcomeStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="space-y-6 text-center">
      <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg">
        <Wifi className="h-10 w-10 text-white" />
      </div>
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
          Welcome to your Travel Router
        </h2>
        <p className="mt-3 text-gray-600 dark:text-gray-400">
          This setup wizard will help you configure your OpenWrt travel router in a few easy steps.
          You&apos;ll set a secure password, connect to an upstream WiFi network, and configure your
          own access point.
        </p>
      </div>
      <div className="grid grid-cols-3 gap-4 pt-4">
        <div className="rounded-lg border p-3 dark:border-gray-700">
          <KeyRound className="mx-auto h-6 w-6 text-blue-500" />
          <p className="mt-2 text-xs text-gray-500">Secure Password</p>
        </div>
        <div className="rounded-lg border p-3 dark:border-gray-700">
          <Wifi className="mx-auto h-6 w-6 text-blue-500" />
          <p className="mt-2 text-xs text-gray-500">WiFi Connection</p>
        </div>
        <div className="rounded-lg border p-3 dark:border-gray-700">
          <Shield className="mx-auto h-6 w-6 text-blue-500" />
          <p className="mt-2 text-xs text-gray-500">AP Configuration</p>
        </div>
      </div>
      <Button onClick={onNext} size="lg" className="mt-4 w-full">
        Get Started <Rocket className="ml-2 h-4 w-4" />
      </Button>
    </div>
  );
}
