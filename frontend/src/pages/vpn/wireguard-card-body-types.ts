import type { UseFormReturn } from 'react-hook-form';
import type {
  VpnStatus,
  WireguardConfig,
  WireGuardStatus,
  WireGuardProfile,
  KillSwitchStatus,
} from '@shared/index';
import type { WireguardProfileImportFormValues } from '@/lib/schemas/vpn-forms';

export type WireguardCardBodyProps = {
  wgStatus: VpnStatus | undefined;
  wgLiveStatus: WireGuardStatus | undefined;
  config: WireguardConfig | undefined;
  profiles: readonly WireGuardProfile[];
  killSwitch: KillSwitchStatus | undefined;
  isToggling: boolean;
  desiredEnabled: boolean | undefined;
  statusDetail: string | undefined;
  toggleMutationPending: boolean;
  onToggleWireguard: () => void;
  activateProfileMutation: { mutate: (id: string) => void; isPending: boolean };
  deleteProfileMutation: { mutate: (id: string) => void; isPending: boolean };
  killSwitchMutation: { mutate: (enabled: boolean) => void; isPending: boolean };
  importForm: UseFormReturn<WireguardProfileImportFormValues>;
  onImportSubmit: (data: WireguardProfileImportFormValues) => void;
  onFileSelected: (file: File | null) => void;
  addProfilePending: boolean;
};
