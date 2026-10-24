/**
 * Network hooks, grouped by domain.
 *
 * The implementations live in focused sibling modules so that a change to one
 * area of the network surface does not have to be read through the whole
 * network surface. This module stays the single import path: every consumer
 * keeps importing from '@/hooks/use-network'.
 */
export {
  useNetworkStatus,
  useClients,
  useSetClientAlias,
  useKickClient,
  useBlockClient,
  useUnblockClient,
  useBlockedClients,
  useSetInterfaceState,
} from './use-network-clients';

export {
  useDHCPConfig,
  useDHCPLeases,
  useSetDHCPConfig,
  useDHCPReservations,
  useAddDHCPReservation,
  useDeleteDHCPReservation,
} from './use-network-dhcp';

export {
  useDNSConfig,
  useSetDNSConfig,
  useDNSEntries,
  useAddDNSEntry,
  useDeleteDNSEntry,
  useDDNSConfig,
  useDDNSStatus,
  useSetDDNSConfig,
} from './use-network-dns';

export {
  useFirewallZones,
  usePortForwards,
  useAddPortForward,
  useDeletePortForward,
  useRunDiagnostics,
  useSendWoL,
  useIPv6Status,
  useSetIPv6Enabled,
  useDoHConfig,
  useSetDoHConfig,
} from './use-network-firewall';

export {
  useWanConfig,
  useDetectWanType,
  useConnectionMethod,
  useFailoverConfig,
  useSetFailoverConfig,
  useFailoverEvents,
  useUptimeLog,
} from './use-network-wan';
