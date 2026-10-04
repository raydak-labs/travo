import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { routeWithParam, routeWithSegment } from '@/lib/api-url';
import { finalizeWifiMutation } from '@/lib/wifi-apply';
import { refreshRouterState } from '@/lib/router-state-refresh';
import { API_ROUTES } from '@shared/index';
import type {
  WifiScanResult,
  WifiConnection,
  WifiHealth,
  SavedNetwork,
  WifiMode,
  APConfig,
  APConfigUpdate,
  RepeaterOptions,
  MACConfig,
  GuestWifiConfig,
  RadioInfo,
  NetworkPriorityRequest,
  AutoReconnectConfig,
  RandomizeMACResponse,
  WifiMutationResponse,
  BandSwitchConfig,
  BandSwitchResponse,
  WiFiSchedule,
  MACPolicies,
} from '@shared/index';

export function useWifiScan(enabled = true) {
  return useQuery({
    queryKey: ['wifi', 'scan'],
    queryFn: () => apiClient.get<WifiScanResult[]>(API_ROUTES.wifi.scan),
    enabled,
  });
}

export function useWifiConnection() {
  return useQuery({
    queryKey: ['wifi', 'connection'],
    queryFn: () => apiClient.get<WifiConnection>(API_ROUTES.wifi.connection),
  });
}

export function useWifiHealth() {
  return useQuery({
    queryKey: ['wifi', 'health'],
    queryFn: () => apiClient.get<WifiHealth>(API_ROUTES.wifi.health),
    refetchInterval: 15_000,
  });
}

/**
 * A WiFi change perturbs every `wifi` query plus a few router-wide ones.
 *
 * The broad invalidate covers all `wifi` keys (including ones that need no
 * follow-up, like the radio scan); `refreshRouterState` then only repeats the
 * delayed passes on `refreshKeys`. Calling both in full fetched the listed keys
 * four times per mutation.
 */
function wifiMutationSettled(queryClient: QueryClient, refreshKeys: Array<readonly unknown[]>) {
  void queryClient.invalidateQueries({ queryKey: ['wifi'] });
  for (const queryKey of refreshKeys) {
    void queryClient.invalidateQueries({ queryKey });
  }
  void refreshRouterState(queryClient, refreshKeys, { skipImmediate: true });
}

export function useWifiConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: {
      ssid: string;
      password: string;
      encryption?: string;
      hidden?: boolean;
      /** Preferred band for dual-band networks (2.4ghz, 5ghz, 6ghz) */
      band?: string;
    }) =>
      finalizeWifiMutation(apiClient.post<WifiMutationResponse>(API_ROUTES.wifi.connect, params)),
    onSuccess: (_data, variables) => {
      toast.success(`Connected to ${variables.ssid}`);
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'saved'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      toast.error('WiFi connection failed', { description: error.message });
    },
  });
}

export function useWifiDisconnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      finalizeWifiMutation(apiClient.post<WifiMutationResponse>(API_ROUTES.wifi.disconnect)),
    onSuccess: () => {
      toast.success('Disconnected from WiFi');
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'saved'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      toast.error('Failed to disconnect', { description: error.message });
    },
  });
}

export function useWifiMode() {
  const queryClient = useQueryClient();
  return useMutation({
    // acknowledge_lockout rides along with the mode so the caller can re-send
    // the SAME request once the operator has ticked the lockout dialog. The
    // hook must not decide that: the dialog does, in the page.
    mutationFn: ({
      mode,
      acknowledge_lockout,
    }: {
      mode: WifiMode;
      acknowledge_lockout?: boolean;
    }) =>
      finalizeWifiMutation(
        apiClient.put<WifiMutationResponse>(API_ROUTES.wifi.mode, {
          mode,
          ...(acknowledge_lockout ? { acknowledge_lockout: true } : {}),
        }),
      ),
    onSuccess: (_data, { mode }) => {
      toast.success(`WiFi mode changed to ${mode}`);
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'saved'],
        ['wifi', 'ap'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      // The lockout refusal also raises its own acknowledgement dialog, but it
      // is NOT swallowed here: this hook is shared by pages that do not wire
      // that dialog, and silence there would leave a refused change with no
      // explanation at all.
      toast.error('Failed to change WiFi mode', { description: error.message });
    },
  });
}

export function useSavedNetworks() {
  return useQuery({
    queryKey: ['wifi', 'saved'],
    queryFn: () => apiClient.get<SavedNetwork[]>(API_ROUTES.wifi.saved),
  });
}

export function useWifiDelete() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (section: string) =>
      finalizeWifiMutation(
        apiClient.del<WifiMutationResponse>(routeWithSegment(API_ROUTES.wifi.deleteSaved, section)),
      ),
    onSuccess: () => {
      toast.success('Network removed');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'saved'] });
      void refreshRouterState(queryClient, [['wifi', 'saved']]);
    },
    onError: (error) => {
      toast.error('Failed to remove network', { description: error.message });
    },
  });
}

export function useSetNetworkPriority() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: NetworkPriorityRequest) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.savedPriority, req),
    onSuccess: () => {
      toast.success('Network priority updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'saved'] });
      void refreshRouterState(queryClient, [['wifi', 'saved']]);
    },
    onError: (error) => {
      toast.error('Failed to update priority', { description: error.message });
    },
  });
}

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

export function useRepeaterOptions(enabled = true) {
  return useQuery({
    queryKey: ['wifi', 'repeater-options'],
    queryFn: () => apiClient.get<RepeaterOptions>(API_ROUTES.wifi.repeaterOptions),
    enabled,
  });
}

export function useSetRepeaterOptions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: RepeaterOptions) =>
      finalizeWifiMutation(
        apiClient.put<WifiMutationResponse & Pick<RepeaterOptions, 'allow_ap_on_sta_radio'>>(
          API_ROUTES.wifi.repeaterOptions,
          body,
        ),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'repeater-options'] });
    },
    onError: (error) => {
      toast.error('Failed to save repeater options', { description: error.message });
    },
  });
}

export function useRepeaterRadioReconcile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      finalizeWifiMutation(
        apiClient.post<WifiMutationResponse>(API_ROUTES.wifi.repeaterReconcile, {}),
      ),
    onSuccess: () => {
      toast.success('Repeater radio layout updated');
      wifiMutationSettled(queryClient, [
        ['wifi', 'health'],
        ['wifi', 'ap'],
        ['wifi', 'connection'],
      ]);
    },
    onError: (error) => {
      toast.error('Could not update radio layout', { description: error.message });
    },
  });
}

export function useMACAddresses() {
  return useQuery({
    queryKey: ['wifi', 'mac'],
    queryFn: () => apiClient.get<MACConfig[]>(API_ROUTES.wifi.mac),
  });
}

export function useSetMAC() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) =>
      finalizeWifiMutation(apiClient.put<WifiMutationResponse>(API_ROUTES.wifi.mac, { mac })),
    onSuccess: () => {
      toast.success('MAC address updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'mac'] });
    },
    onError: (error) => {
      toast.error('Failed to update MAC address', { description: error.message });
    },
  });
}

export function useRandomizeMAC() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      finalizeWifiMutation(apiClient.post<RandomizeMACResponse>(API_ROUTES.wifi.macRandomize)),
    onSuccess: (data) => {
      toast.success(`MAC randomized to ${data.mac}`);
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'mac'] });
    },
    onError: (error) => {
      toast.error('Failed to randomize MAC', { description: error.message });
    },
  });
}

export function useGuestWifi() {
  return useQuery({
    queryKey: ['wifi', 'guest'],
    queryFn: () => apiClient.get<GuestWifiConfig>(API_ROUTES.wifi.guest),
  });
}

export function useSetGuestWifi() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: GuestWifiConfig) =>
      finalizeWifiMutation(apiClient.put<WifiMutationResponse>(API_ROUTES.wifi.guest, cfg)),
    onSuccess: () => {
      toast.success('Guest WiFi updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'guest'] });
    },
    onError: (error) => {
      toast.error('Failed to update guest WiFi', { description: error.message });
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

export function useAutoReconnect() {
  return useQuery({
    queryKey: ['wifi', 'autoreconnect'],
    queryFn: () => apiClient.get<AutoReconnectConfig>(API_ROUTES.wifi.autoreconnect),
  });
}

export function useSetAutoReconnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.autoreconnect, { enabled }),
    onSuccess: (_data, enabled) => {
      toast.success(enabled ? 'Auto-reconnect enabled' : 'Auto-reconnect disabled');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'autoreconnect'] });
    },
    onError: (error) => {
      toast.error('Failed to update auto-reconnect', { description: error.message });
    },
  });
}

export function useBandSwitching() {
  return useQuery({
    queryKey: ['wifi', 'band-switching'],
    queryFn: () => apiClient.get<BandSwitchResponse>(API_ROUTES.wifi.bandSwitching),
    refetchInterval: 10_000,
  });
}

export function useSetBandSwitching() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: BandSwitchConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.bandSwitching, config),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'band-switching'] });
    },
    onError: (error) => {
      toast.error('Failed to update band switching config', { description: error.message });
    },
  });
}

export function useWiFiSchedule() {
  return useQuery({
    queryKey: ['wifi', 'schedule'],
    queryFn: () => apiClient.get<WiFiSchedule>(API_ROUTES.wifi.schedule),
  });
}

export function useSetWiFiSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (schedule: WiFiSchedule) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.schedule, schedule),
    onSuccess: () => {
      toast.success('WiFi schedule saved');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'schedule'] });
    },
    onError: (error) => {
      toast.error('Failed to save WiFi schedule', { description: error.message });
    },
  });
}

export function useMACPolicies() {
  return useQuery({
    queryKey: ['wifi', 'macPolicies'],
    queryFn: () => apiClient.get<MACPolicies>(API_ROUTES.wifi.macPolicies),
  });
}

export function useSetMACPolicies() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (policies: MACPolicies) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.macPolicies, policies),
    onSuccess: () => {
      toast.success('MAC policies saved');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'macPolicies'] });
    },
    onError: (error) => {
      toast.error('Failed to save MAC policies', { description: error.message });
    },
  });
}
