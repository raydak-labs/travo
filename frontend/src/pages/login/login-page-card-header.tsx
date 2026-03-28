import { Wifi } from 'lucide-react';
import { CardHeader, CardTitle, CardDescription } from '@/components/ui/card';

export function LoginPageCardHeader() {
  return (
    <CardHeader className="space-y-4 pb-2 text-center">
      <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-600 shadow-md">
        <Wifi className="h-8 w-8 text-white" />
      </div>
      <div>
        <CardTitle className="text-2xl">Travel Router</CardTitle>
        <CardDescription className="mt-2">OpenWrt Travel Router Management</CardDescription>
      </div>
    </CardHeader>
  );
}
