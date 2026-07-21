import { Smartphone, CheckCircle, XCircle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  useUSBTetherStatus,
  useConfigureUSBTether,
  useUnconfigureUSBTether,
} from '@/hooks/use-usb-tether';

export function USBTetheringSection() {
  const { data: status } = useUSBTetherStatus();
  const configureMutation = useConfigureUSBTether();
  const unconfigureMutation = useUnconfigureUSBTether();

  // Don't show the section if no device is detected and no UCI config exists.
  if (!status?.detected && !status?.configured) return null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>USB Tethering</CardTitle>
        <Smartphone className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-3">
        {status?.detected ? (
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-green-500" />
              <span className="text-sm font-medium">
                {status.device_type === 'android'
                  ? 'Android'
                  : status.device_type === 'ios'
                    ? 'iOS'
                    : 'USB'}{' '}
                device detected
              </span>
              <Badge variant="outline" className="font-mono text-xs">
                {status.interface}
              </Badge>
              {status.is_up ? (
                <Badge variant="success">Up</Badge>
              ) : (
                <Badge variant="secondary">Down</Badge>
              )}
            </div>
            {status.ip_address && (
              <p className="text-sm text-gray-500 dark:text-gray-400 font-mono">{status.ip_address}</p>
            )}
            {status.configured ? (
              <div className="flex items-center gap-2">
                <Badge variant="default">Configured as WAN</Badge>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => unconfigureMutation.mutate()}
                  disabled={unconfigureMutation.isPending}
                >
                  Remove WAN config
                </Button>
              </div>
            ) : (
              <Button
                size="sm"
                onClick={() => configureMutation.mutate(status.interface)}
                disabled={configureMutation.isPending}
              >
                Use as WAN source
              </Button>
            )}
          </div>
        ) : (
          <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <XCircle className="h-4 w-4" />
            <span>No USB tethering device detected (UCI config still active)</span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => unconfigureMutation.mutate()}
              disabled={unconfigureMutation.isPending}
            >
              Remove config
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
