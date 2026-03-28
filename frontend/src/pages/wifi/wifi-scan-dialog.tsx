import { useState } from 'react';
import { Search } from 'lucide-react';
import type { GroupedScanNetwork } from '@shared/index';
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
import { useWifiScan, useWifiConnect, useWifiConnection } from '@/hooks/use-wifi';

export function WifiScanDialog() {
  const [open, setOpen] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<GroupedScanNetwork | null>(null);
  const { data: scanResults = [], isLoading, refetch } = useWifiScan(!open ? false : true);
  const { data: connection } = useWifiConnection();
  const connectMutation = useWifiConnect();

  function handleConnect(ssid: string, password: string, band?: string) {
    connectMutation.mutate(
      {
        ssid,
        password,
        encryption: selectedGroup?.encryption,
        band: band ?? undefined,
      },
      {
        onSuccess: () => {
          setSelectedGroup(null);
          setOpen(false);
        },
      },
    );
  }

  return (
    <>
      <Button
        onClick={() => {
          setOpen(true);
          void refetch();
        }}
        size="sm"
      >
        <Search className="mr-1.5 h-3.5 w-3.5" />
        Scan Networks
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[80vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Available Networks</DialogTitle>
            <DialogDescription>Select a network to connect to.</DialogDescription>
          </DialogHeader>

          {selectedGroup ? (
            <WifiConnectDialog
              group={selectedGroup}
              isConnecting={connectMutation.isPending}
              error={connectMutation.error?.message ?? null}
              onConnect={handleConnect}
              onCancel={() => setSelectedGroup(null)}
              embedded
            />
          ) : (
            <WifiScanList
              networks={scanResults}
              isLoading={isLoading}
              onRefresh={() => void refetch()}
              onConnect={setSelectedGroup}
              connectedSSID={connection?.connected ? connection.ssid : null}
              connectedBand={connection?.band ?? null}
            />
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
