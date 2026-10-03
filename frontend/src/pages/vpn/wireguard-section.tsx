import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Shield } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
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
  const vpnStatusQuery = useVpnStatus();
  const servicesQuery = useServices();
  const configQuery = useWireguardConfig();
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

  const wgStatus = vpnStatusQuery.data?.find((v) => v.type === 'wireguard');
  const wgService = servicesQuery.data?.find((s) => s.id === 'wireguard');
  // "Not installed" may only be concluded from a service list or VPN status that
  // actually arrived; a failed GET must not read as an absent feature.
  const isInstalled = wgService ? wgService.state !== 'not_installed' : !!wgStatus;
  // Both the service list and the VPN status feed the install state. It can only
  // be concluded once both have settled; if either request failed, "not
  // installed" would be a guess, and the install link would point at an action
  // that cannot work.
  const installStatePending =
    !(servicesQuery.isSuccess || servicesQuery.isError) ||
    !(vpnStatusQuery.isSuccess || vpnStatusQuery.isError);
  const installStateFailed = servicesQuery.isError || vpnStatusQuery.isError;
  const installStateError = servicesQuery.error ?? vpnStatusQuery.error;
  const configFailed = isInstalled && configQuery.isError;
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

  const retryInstallState = () => {
    void servicesQuery.refetch();
    void vpnStatusQuery.refetch();
    void configQuery.refetch();
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
          <CardTitle>WireGuard</CardTitle>
          <Shield className="h-4 w-4 text-blue-500" />
        </CardHeader>
        <CardContent className="space-y-4">
          <QueryCard
            isLoading={installStatePending || (isInstalled && configQuery.isLoading)}
            isError={installStateFailed || configFailed}
            error={installStateError ?? configQuery.error}
            onRetry={retryInstallState}
            loading={
              <div className="space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            }
          >
            {!isInstalled ? (
              <WireguardInstallPrompt />
            ) : (
              <WireguardCardBody
                wgStatus={wgStatus}
                wgLiveStatus={wgLiveStatus}
                config={configQuery.data}
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
          </QueryCard>
        </CardContent>
      </Card>
    </>
  );
}
