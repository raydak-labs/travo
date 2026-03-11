import { useState } from 'react';
import { Search } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { WifiScanList } from './wifi-scan-list';
import { WifiConnectDialog } from './wifi-connect-dialog';
import { useWifiScan, useWifiConnect } from '@/hooks/use-wifi';

export function WifiScanDialog() {
  const [open, setOpen] = useState(false);
  const [selectedNetwork, setSelectedNetwork] = useState<WifiScanResult | null>(null);
  const { data: scanResults = [], isLoading, refetch } = useWifiScan(!open ? false : true);
  const connectMutation = useWifiConnect();

  function handleOpen() {
    setOpen(true);
    void refetch();
  }

  function handleConnect(ssid: string, password: string) {
    connectMutation.mutate(
      { ssid, password },
      {
        onSuccess: () => {
          setSelectedNetwork(null);
          setOpen(false);
        },
      },
    );
  }

  return (
    <>
      <Button onClick={handleOpen} size="sm">
        <Search className="mr-1.5 h-3.5 w-3.5" />
        Scan Networks
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[80vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Available Networks</DialogTitle>
            <DialogDescription>Select a network to connect to.</DialogDescription>
          </DialogHeader>

          {selectedNetwork ? (
            <WifiConnectDialog
              network={selectedNetwork}
              isConnecting={connectMutation.isPending}
              error={connectMutation.error?.message ?? null}
              onConnect={handleConnect}
              onCancel={() => setSelectedNetwork(null)}
              embedded
            />
          ) : (
            <WifiScanList
              networks={scanResults}
              isLoading={isLoading}
              onRefresh={() => void refetch()}
              onConnect={setSelectedNetwork}
            />
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
