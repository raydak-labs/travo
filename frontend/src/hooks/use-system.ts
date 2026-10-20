/**
 * System hooks, grouped by domain.
 *
 * The implementations live in focused sibling modules so that a change to one
 * area of the system surface does not have to be read through the whole
 * system surface. This module stays the single import path: every consumer
 * keeps importing from '@/hooks/use-system'.
 */
export {
  useSystemInfo,
  useSystemStats,
  statsRefetchInterval,
  useSystemLogs,
  useKernelLogs,
  useRunSpeedTest,
} from './use-system-stats';

export {
  useReboot,
  useShutdown,
  useFactoryReset,
  useFirmwareUpgrade,
  useBackup,
  useRestore,
} from './use-system-power';

export {
  useSetHostname,
  useTimezone,
  useSetTimezone,
  useNTPConfig,
  useSyncNTP,
  useSetNTPConfig,
  useLEDStatus,
  useSetLEDStealth,
  useLEDSchedule,
  useSetLEDSchedule,
  useHardwareButtons,
  useSetButtonActions,
  useSSHKeys,
  useAddSSHKey,
  useDeleteSSHKey,
} from './use-system-network';

export {
  useChangePassword,
  useAlertThresholds,
  useSetAlertThresholds,
  useCompleteSetup,
} from './use-system-account';
