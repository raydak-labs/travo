import { Shield } from 'lucide-react';

export function APStepIntro() {
  return (
    <div className="text-center">
      <Shield className="mx-auto h-10 w-10 text-blue-500" />
      <h2 className="mt-3 text-xl font-bold text-gray-900 dark:text-white">
        Configure Access Point
      </h2>
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Set up your router&apos;s WiFi network. Your devices will connect to this network.
      </p>
    </div>
  );
}
