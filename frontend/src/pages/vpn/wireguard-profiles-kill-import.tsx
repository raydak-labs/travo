import type { UseFormReturn } from 'react-hook-form';
import { ShieldAlert, Trash2, Play } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import type { WireGuardProfile, KillSwitchStatus } from '@shared/index';
import type { WireguardProfileImportFormValues } from '@/lib/schemas/vpn-forms';
import { WireguardImportProfileForm } from './wireguard-import-profile-form';

export type WireguardProfilesKillImportProps = {
  profiles: readonly WireGuardProfile[];
  killSwitch: KillSwitchStatus | undefined;
  activateProfileMutation: { mutate: (id: string) => void; isPending: boolean };
  deleteProfileMutation: { mutate: (id: string) => void; isPending: boolean };
  killSwitchMutation: { mutate: (enabled: boolean) => void; isPending: boolean };
  importForm: UseFormReturn<WireguardProfileImportFormValues>;
  onImportSubmit: (data: WireguardProfileImportFormValues) => void;
  onFileSelected: (file: File | null) => void;
  addProfilePending: boolean;
};

export function WireguardProfilesKillImport({
  profiles,
  killSwitch,
  activateProfileMutation,
  deleteProfileMutation,
  killSwitchMutation,
  importForm,
  onImportSubmit,
  onFileSelected,
  addProfilePending,
}: WireguardProfilesKillImportProps) {
  return (
    <>
      {profiles.length > 0 && (
        <div>
          <h4 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
            Profiles ({profiles.length})
          </h4>
          <ul className="space-y-2" role="list" aria-label="WireGuard profiles">
            {profiles.map((profile) => (
              <li
                key={profile.id}
                className={`flex items-center justify-between rounded-md border p-3 text-sm ${
                  profile.active
                    ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-950'
                    : 'border-gray-200 dark:border-gray-700'
                }`}
              >
                <div className="flex items-center gap-2">
                  <span className="font-medium text-gray-900 dark:text-white">{profile.name}</span>
                  {profile.active && <Badge variant="success">Active</Badge>}
                </div>
                <div className="flex items-center gap-1">
                  {!profile.active && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => activateProfileMutation.mutate(profile.id)}
                      disabled={activateProfileMutation.isPending}
                      title="Activate profile"
                    >
                      <Play className="h-4 w-4" />
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => deleteProfileMutation.mutate(profile.id)}
                    disabled={deleteProfileMutation.isPending}
                    title="Delete profile"
                  >
                    <Trash2 className="h-4 w-4 text-red-500" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="rounded-md border border-gray-200 p-3 dark:border-gray-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ShieldAlert className="h-4 w-4 text-orange-500" />
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Kill Switch</span>
          </div>
          <Switch
            id="killswitch-toggle"
            checked={killSwitch?.enabled ?? false}
            onChange={() => killSwitchMutation.mutate(!(killSwitch?.enabled ?? false))}
            disabled={killSwitchMutation.isPending}
          />
        </div>
        <p className="mt-1 text-xs text-gray-500">
          {killSwitch?.enabled
            ? 'All traffic is blocked if VPN disconnects. Disable to allow direct internet access.'
            : 'When enabled, blocks all internet traffic if the VPN connection drops to prevent IP leaks.'}
        </p>
      </div>

      <WireguardImportProfileForm
        form={importForm}
        onSubmit={onImportSubmit}
        onFileSelected={onFileSelected}
        isSaving={addProfilePending}
      />
    </>
  );
}
