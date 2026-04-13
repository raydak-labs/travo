import { useState } from 'react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Activity,
  Download,
  Upload,
  Clock,
  Server,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Package,
  Trash2,
  Loader2,
} from 'lucide-react';
import { toast } from 'sonner';
import {
  useSpeedtestService,
  useInstallSpeedtestCLI,
  useUninstallSpeedtestCLI,
  useRunSpeedtestCLI,
} from '@/hooks/use-system';
import type { SpeedTestResult } from '@shared/index';

export function SystemSpeedtestServiceCard() {
  const [testResult, setTestResult] = useState<SpeedTestResult | null>(null);
  const [isRunning, setIsRunning] = useState(false);
  
  const { data: service, isLoading: isLoadingService } = useSpeedtestService();
  const installMutation = useInstallSpeedtestCLI();
  const uninstallMutation = useUninstallSpeedtestCLI();
  const runMutation = useRunSpeedtestCLI();

  const handleInstall = () => {
    installMutation.mutate();
  };

  const handleUninstall = () => {
    uninstallMutation.mutate();
  };

  const handleRun = () => {
    setIsRunning(true);
    setTestResult(null);
    
    runMutation.mutate(undefined, {
      onSuccess: (result) => {
        setTestResult(result);
        setIsRunning(false);
      },
      onError: () => {
        setIsRunning(false);
      },
    });
  };

  if (isLoadingService) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Speedtest Service</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin" />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!service) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Speedtest Service</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center text-sm text-gray-500 py-4">
            Unable to load speedtest service status
          </div>
        </CardContent>
      </Card>
    );
  }

  const getStatusBadge = () => {
    if (!service.supported) {
      return (
        <Badge variant="destructive" className="gap-2">
          <XCircle className="h-4 w-4" />
          Not Supported
        </Badge>
      );
    }
    if (service.installed) {
      return (
        <Badge variant="default" className="gap-2">
          <CheckCircle2 className="h-4 w-4" />
          Installed
        </Badge>
      );
    }
    return (
      <Badge variant="secondary" className="gap-2">
        <AlertTriangle className="h-4 w-4" />
        Not Installed
      </Badge>
    );
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Speedtest Service
          </CardTitle>
          {getStatusBadge()}
        </div>
        <CardDescription>
          {service.supported
            ? `Run internet speed tests using Speedtest.net CLI. Architecture: ${service.architecture}`
            : `${service.architecture} architecture is not supported by Speedtest.net CLI`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <div className="text-gray-500">Architecture</div>
            <div className="font-medium">{service.architecture}</div>
          </div>
          <div>
            <div className="text-gray-500">Package</div>
            <div className="font-medium">{service.package_name}</div>
          </div>
          {service.version && (
            <div>
              <div className="text-gray-500">Version</div>
              <div className="font-medium">{service.version}</div>
            </div>
          )}
          <div>
            <div className="text-gray-500">Storage Required</div>
            <div className="font-medium">~{service.storage_size_mb} MB</div>
          </div>
        </div>

        {!service.installed && service.supported && (
          <div className="flex gap-2">
            <Button
              onClick={handleInstall}
              disabled={installMutation.isPending}
              className="flex-1"
            >
              <Package className="h-4 w-4 mr-2" />
              {installMutation.isPending ? 'Installing...' : 'Install Speedtest CLI'}
            </Button>
          </div>
        )}

        {service.installed && (
          <div className="space-y-4">
            <Button
              onClick={handleRun}
              disabled={isRunning}
              className="w-full"
            >
              {isRunning ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Running Speed Test...
                </>
              ) : (
                <>
                  <Activity className="h-4 w-4 mr-2" />
                  Run Speed Test
                </>
              )}
            </Button>

            <Button
              variant="destructive"
              onClick={handleUninstall}
              disabled={uninstallMutation.isPending}
              className="w-full"
            >
              <Trash2 className="h-4 w-4 mr-2" />
              {uninstallMutation.isPending ? 'Uninstalling...' : 'Uninstall'}
            </Button>

            {testResult && (
              <div className="rounded-lg border bg-card p-4 space-y-3">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <CheckCircle2 className="h-5 w-5 text-green-500" />
                  Speed Test Complete
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2 text-sm text-gray-500">
                      <Download className="h-4 w-4" />
                      Download
                    </div>
                    <div className="text-2xl font-bold">
                      {testResult.download_mbps.toFixed(2)}
                    </div>
                    <div className="text-xs text-gray-500">Mbps</div>
                  </div>
                  <div className="space-y-1">
                    <div className="flex items-center gap-2 text-sm text-gray-500">
                      <Upload className="h-4 w-4" />
                      Upload
                    </div>
                    <div className="text-2xl font-bold">
                      {testResult.upload_mbps.toFixed(2)}
                    </div>
                    <div className="text-xs text-gray-500">Mbps</div>
                  </div>
                  <div className="space-y-1">
                    <div className="flex items-center gap-2 text-sm text-gray-500">
                      <Clock className="h-4 w-4" />
                      Ping
                    </div>
                    <div className="text-2xl font-bold">
                      {testResult.ping_ms.toFixed(1)}
                    </div>
                    <div className="text-xs text-gray-500">ms</div>
                  </div>
                </div>
                {testResult.server && (
                  <div className="flex items-center gap-2 text-sm text-gray-500 pt-2">
                    <Server className="h-4 w-4" />
                    Server: {testResult.server}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {!service.supported && (
          <div className="rounded-lg bg-destructive/10 p-4 text-sm text-destructive">
            Your device architecture ({service.architecture}) is not supported by the Speedtest.net CLI.
            Only these architectures are supported: aarch64, arm, x86_64, i386, mips, mipsel.
          </div>
        )}
      </CardContent>
    </Card>
  );
}
