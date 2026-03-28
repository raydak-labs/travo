import { Wifi } from 'lucide-react';

export function WifiStepIntro() {
  return (
    <div className="text-center">
      <Wifi className="mx-auto h-10 w-10 text-blue-500" />
      <h2 className="mt-3 text-xl font-bold text-gray-900 dark:text-white">Connect to WiFi</h2>
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Select the upstream WiFi network you want your router to connect to (hotel, cafe, etc.).
      </p>
    </div>
  );
}
