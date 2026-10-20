/**
 * WiFi hooks, grouped by domain.
 *
 * The implementations live in focused sibling modules so that a change to one
 * area of the wireless surface does not have to be read through the whole
 * wireless surface. This module stays the single import path: every consumer
 * keeps importing from '@/hooks/use-wifi'.
 */
export {
  useWifiScan,
  useWifiConnection,
  useWifiHealth,
  useWifiConnect,
  useWifiDisconnect,
  useWifiMode,
  useSavedNetworks,
  useWifiDelete,
  useSetNetworkPriority,
} from './use-wifi-connection';

export { useAPConfigs, useSetAPConfig, useRadioStatus, useRadios, useSetRadioRole, useSetRadioEnabled } from './use-wifi-ap';

export {
  useRepeaterOptions,
  useSetRepeaterOptions,
  useRepeaterRadioReconcile,
  useMACAddresses,
  useSetMAC,
  useRandomizeMAC,
  useGuestWifi,
  useSetGuestWifi,
  useAutoReconnect,
  useSetAutoReconnect,
  useBandSwitching,
  useSetBandSwitching,
  useWiFiSchedule,
  useSetWiFiSchedule,
  useMACPolicies,
  useSetMACPolicies,
} from './use-wifi-network';
