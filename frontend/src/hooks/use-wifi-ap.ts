/**
 * The access-point side of the radio: the AP configurations per section, the
 * radio inventory, the radio role, and the radio on/off switch.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { routeWithParam, routeWithSegment } from '@/lib/api-url';
import { finalizeWifiMutation } from '@/lib/wifi-apply';
import { refreshRouterState } from '@/lib/router-state-refresh';
import { API_ROUTES } from '@shared/index';
import type { APConfig, APConfigUpdate, RadioInfo, WifiMutationResponse } from '@shared/index';
import { wifiMutationSettled } from './use-wifi-settled';

export function useAPConfigs() {
  return useQuery({
    queryKey: ['wifi', 'ap'],
    queryFn: () => apiClient.get<APConfig[]>(API_ROUTES.wifi.ap),
  });
}

export function useSetAPConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ section, config }: { section: string; config: APConfigUpdate }) =>
      finalizeWifiMutation(
        apiClient.put<WifiMutationResponse>(routeWithSegment(API_ROUTES.wifi.ap, section), config),
      ),
    onSuccess: () => {
      toast.success('AP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'ap'] });
      void refreshRouterState(queryClient, [
        ['wifi', 'health'],
        ['wifi', 'connection'],
      ]);
    },
    onError: (error) => {
      // Not swallowed for the same reason as useWifiMode's onError.
      toast.error('Failed to update AP config', { description: error.message });
    },
  });
}

export function useRadioStatus() {
  return useQuery({
    queryKey: ['wifi', 'radio'],
    queryFn: () => apiClient.get<{ enabled: boolean }>(API_ROUTES.wifi.radio),
  });
}

export function useRadios() {
  return useQuery({
    queryKey: ['wifi', 'radios'],
    queryFn: () => apiClient.get<RadioInfo[]>(API_ROUTES.wifi.radios),
  });
}

/**
 * Response of the radio-role endpoint: the shared envelope plus, when the
 * service had to invent a WPA passphrase for a default AP it created,
 * generated_key. The passphrase is returned exactly once and is unrecoverable
 * afterwards, so callers must surface it.
 */
type RadioRoleMutationResponse = WifiMutationResponse & {
  readonly generated_key?: string;
};

export function useSetRadioRole() {
  const queryClient = useQueryClient();
  return useMutation({
    // acknowledge_lockout rides along so a refused role change can be re-sent
    // once the operator has ticked the lockout dialog (ADR 0002 §5).
    mutationFn: ({
      name,
      role,
      acknowledge_lockout: acknowledge,
    }: {
      name: string;
      role: string;
      acknowledge_lockout?: boolean;
    }) =>
      finalizeWifiMutation(
        apiClient.put<RadioRoleMutationResponse>(routeWithParam(API_ROUTES.wifi.radioRole, name), {
          role,
          ...(acknowledge ? { acknowledge_lockout: true } : {}),
        }),
      ),
    onSuccess: (data, { name }) => {
      // A radio switched to AP with no key gets a passphrase invented by the
      // service. It is returned exactly once, in this response, and the new AP
      // on the air uses it — so it has to reach the operator here or the network
      // ends up with a password nobody knows.
      if (data.generated_key) {
        toast.success(`Radio ${name} updated`, {
          description: `Generated WiFi password for ${name}: ${data.generated_key}`,
        });
      } else {
        toast.success('Radio role updated');
      }
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'radios'],
        ['wifi', 'ap'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      // Keep the backend's own wording: a refused role names both remedies, and
      // the radio card repeats the same message inline next to the selector.
      toast.error('Failed to update radio role', { description: error.message });
    },
  });
}

export function useSetRadioEnabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      finalizeWifiMutation(apiClient.put<WifiMutationResponse>(API_ROUTES.wifi.radio, { enabled })),
    onSuccess: () => {
      toast.success('WiFi radio updated');
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'radios'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      toast.error('Failed to update WiFi radio', { description: error.message });
    },
  });
}
