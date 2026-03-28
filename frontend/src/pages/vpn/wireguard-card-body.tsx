import type { WireguardCardBodyProps } from './wireguard-card-body-types';
import { WireguardStatusAndConfigPeers } from './wireguard-status-and-config-peers';
import { WireguardProfilesKillImport } from './wireguard-profiles-kill-import';

export type { WireguardCardBodyProps } from './wireguard-card-body-types';

export function WireguardCardBody({
  wgStatus,
  wgLiveStatus,
  config,
  profiles,
  killSwitch,
  isToggling,
  desiredEnabled,
  statusDetail,
  toggleMutationPending,
  onToggleWireguard,
  activateProfileMutation,
  deleteProfileMutation,
  killSwitchMutation,
  importForm,
  onImportSubmit,
  onFileSelected,
  addProfilePending,
}: WireguardCardBodyProps) {
  return (
    <>
      <WireguardStatusAndConfigPeers
        wgStatus={wgStatus}
        wgLiveStatus={wgLiveStatus}
        config={config}
        isToggling={isToggling}
        desiredEnabled={desiredEnabled}
        statusDetail={statusDetail}
        toggleMutationPending={toggleMutationPending}
        onToggleWireguard={onToggleWireguard}
      />
      <WireguardProfilesKillImport
        profiles={profiles}
        killSwitch={killSwitch}
        activateProfileMutation={activateProfileMutation}
        deleteProfileMutation={deleteProfileMutation}
        killSwitchMutation={killSwitchMutation}
        importForm={importForm}
        onImportSubmit={onImportSubmit}
        onFileSelected={onFileSelected}
        addProfilePending={addProfilePending}
      />
    </>
  );
}
