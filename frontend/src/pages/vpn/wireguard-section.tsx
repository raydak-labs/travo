import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Shield } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { OperationProgressDialog } from '@/components/ui/operation-progress-dialog';
import { useServices } from '@/hooks/use-services';
import {
  useWireguardConfig,
  useToggleWireguard,
  useVpnStatus,
  useWireguardStatus,
  useWireguardProfiles,
  useAddWireguardProfile,
  useDeleteWireguardProfile,
  useActivateWireguardProfile,
  useKillSwitch,
  useSetKillSwitch,
} from '@/hooks/use-vpn';
import {
  wireguardProfileImportFormSchema,
  type WireguardProfileImportFormValues,
} from '@/lib/schemas/vpn-forms';
import { WireguardCardBody } from '@/pages/vpn/wireguard-card-body';
import { applyWireguardImportFile } from '@/pages/vpn/wireguard-import-profile-file';
import { WireguardInstallPrompt } from '@/pages/vpn/wireguard-install-prompt';

export function WireguardSection() {
  const { data: vpnStatuses = [] } = useVpnStatus();
  const { data: services = [] } = useServices();
  const { data: config, isLoading } = useWireguardConfig();
  const { data: wgLiveStatus } = useWireguardStatus();
  const { data: profiles = [] } = useWireguardProfiles();
  const toggleMutation = useToggleWireguard();
  const addProfileMutation = useAddWireguardProfile();
  const deleteProfileMutation = useDeleteWireguardProfile();
  const activateProfileMutation = useActivateWireguardProfile();
  const { data: killSwitch } = useKillSwitch();
  const killSwitchMutation = useSetKillSwitch();

  const importForm = useForm<WireguardProfileImportFormValues>({
    resolver: zodResolver(wireguardProfileImportFormSchema),
    defaultValues: { name: '', config: '' },
    mode: 'onChange',
  });

  const wgStatus = vpnStatuses.find((v) => v.type === 'wireguard');
  const wgService = services.find((s) => s.id === 'wireguard');
  const isInstalled = wgService ? wgService.state !== 'not_installed' : !!wgStatus;
  const isToggling = toggleMutation.isPending;
  const desiredEnabled = isToggling ? toggleMutation.variables : undefined;

  const statusDetail = wgStatus?.status_detail;

  const onImportSubmit = (data: WireguardProfileImportFormValues) => {
    addProfileMutation.mutate(
      { name: data.name.trim(), config: data.config.trim() },
      {
        onSuccess: () => {
          importForm.reset({ name: '', config: '' });
        },
      },
    );
  };

  const handleFileUpload = (file: File | null) => {
    void applyWireguardImportFile(file, importForm);
  };

  return (
    <>
      <OperationProgressDialog
        open={isToggling}
        title={desiredEnabled ? 'Enabling WireGuard…' : 'Disabling WireGuard…'}
        description="Applying network and firewall changes. This may take a few seconds."
        details={[
          'Updating UCI configuration',
          'Applying changes via netifd',
          desiredEnabled
            ? 'Bringing up wg0 and verifying status'
            : 'Tearing down wg0 and restoring uplink routing',
        ]}
      />
      <Card className={!isInstalled ? 'opacity-60' : undefined}>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">WireGuard</CardTitle>
          <Shield className="h-4 w-4 text-blue-500" />
        </CardHeader>
        <CardContent className="space-y-4">
          {!isInstalled ? (
            <WireguardInstallPrompt />
          ) : isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : (
            <WireguardCardBody
              wgStatus={wgStatus}
              wgLiveStatus={wgLiveStatus}
              config={config}
              profiles={profiles}
              killSwitch={killSwitch}
              isToggling={isToggling}
              desiredEnabled={desiredEnabled}
              statusDetail={statusDetail}
              toggleMutationPending={toggleMutation.isPending}
              onToggleWireguard={() => toggleMutation.mutate(!(wgStatus?.enabled ?? false))}
              activateProfileMutation={activateProfileMutation}
              deleteProfileMutation={deleteProfileMutation}
              killSwitchMutation={killSwitchMutation}
              importForm={importForm}
              onImportSubmit={onImportSubmit}
              onFileSelected={handleFileUpload}
              addProfilePending={addProfileMutation.isPending}
            />
          )}
        </CardContent>
      </Card>
    </>
  );
}
