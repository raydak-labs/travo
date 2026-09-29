import { useState, useCallback, useMemo } from 'react';
import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import {
  useWifiScan,
  useWifiConnect,
  useWifiMode,
  useWifiConnection,
  useAPConfigs,
  useSetAPConfig,
  useRepeaterOptions,
  useSetRepeaterOptions,
} from '@/hooks/use-wifi';
import type { RepeaterWizardStep, RepeaterUpstreamConfig, RepeaterApFormConfig } from './types';
import { mapScanEncryptionToUci } from './map-encryption';

const emptyApForm = (): RepeaterApFormConfig => ({
  ssid: '',
  encryption: 'psk2',
  key: '',
  sameAsUpstream: true,
  separateBandConfig: false,
  perBand: {},
});

export function useRepeaterWizard(open: boolean) {
  const [step, setStep] = useState<RepeaterWizardStep>('select-upstream');
  const [selectedNetwork, setSelectedNetwork] = useState<WifiScanResult | null>(null);
  const [upstream, setUpstream] = useState<RepeaterUpstreamConfig>({
    ssid: '',
    password: '',
    encryption: '',
  });
  const [apConfig, setApConfig] = useState<RepeaterApFormConfig>(emptyApForm);
  const [allowApOnStaRadio, setAllowApOnStaRadio] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);
  const [failedStep, setFailedStep] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const { data: scanResults = [], isLoading: scanLoading, refetch } = useWifiScan(open);
  const { data: apConfigs } = useAPConfigs();
  const { data: wifiConn } = useWifiConnection();
  const { data: repeaterOpts } = useRepeaterOptions(open);
  const connectMutation = useWifiConnect();
  const modeMutation = useWifiMode();
  const setAPMutation = useSetAPConfig();
  const setRepeaterOptsMutation = useSetRepeaterOptions();
  const [repeaterOptsHydrated, setRepeaterOptsHydrated] = useState(false);
  const [prevOpen, setPrevOpen] = useState(open);

  // Reset hydration when the dialog closes (see React docs: adjusting state when a prop changes).
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (!open) {
      setRepeaterOptsHydrated(false);
    }
  }

  // Seed from cached repeater options once per open session; `reset()` sets hydrated true to
  // avoid re-seeding while the dialog stays open.
  if (open && repeaterOpts != null && !repeaterOptsHydrated) {
    setAllowApOnStaRadio(repeaterOpts.allow_ap_on_sta_radio);
    setRepeaterOptsHydrated(true);
  }

  const reset = useCallback(() => {
    setStep('select-upstream');
    setSelectedNetwork(null);
    setUpstream({ ssid: '', password: '', encryption: '' });
    setApConfig(emptyApForm());
    setAllowApOnStaRadio(false);
    // Avoid immediately re-seeding from cached repeaterOpts while the dialog is still open;
    // `open === false` clears this so the next open hydrates fresh.
    setRepeaterOptsHydrated(true);
    setApplying(false);
    setApplyError(null);
    setFailedStep(null);
    setDone(false);
  }, []);

  const handleSelectNetwork = useCallback((group: GroupedScanNetwork) => {
    const first = group.aps[0];
    setSelectedNetwork(first);
    setUpstream({
      ssid: group.ssid,
      password: '',
      encryption: group.encryption,
    });
    setApConfig((prev) => ({
      ...prev,
      ssid: prev.sameAsUpstream ? group.ssid : prev.ssid,
    }));
  }, []);

  const effectiveAPSSID = apConfig.sameAsUpstream ? upstream.ssid : apConfig.ssid;
  const effectiveAPKey = apConfig.sameAsUpstream ? upstream.password : apConfig.key;
  const effectiveAPEncryption = apConfig.sameAsUpstream
    ? mapScanEncryptionToUci(upstream.encryption)
    : apConfig.encryption;

  const apSummaryLine = useMemo(() => {
    if (apConfig.sameAsUpstream) {
      return upstream.ssid;
    }
    if (apConfig.separateBandConfig && apConfigs?.length) {
      return apConfigs
        .map((ap) => {
          const pb = apConfig.perBand[ap.section];
          const label = ap.band === '2g' ? '2.4 GHz' : ap.band === '5g' ? '5 GHz' : ap.band;
          return `${label}: ${pb?.ssid?.trim() || apConfig.ssid}`;
        })
        .join(' · ');
    }
    return apConfig.ssid || effectiveAPSSID;
  }, [
    apConfig.sameAsUpstream,
    apConfig.separateBandConfig,
    apConfig.perBand,
    apConfig.ssid,
    apConfigs,
    effectiveAPSSID,
    upstream.ssid,
  ]);

  const handleApply = useCallback(async () => {
    setApplying(true);
    setApplyError(null);
    setFailedStep(null);

    // Snapshot the current radio state so a mid-sequence failure can put the
    // device back where it was (see `rollback` below).
    const previousMode = wifiConn?.mode ?? null;
    const previousApConfigs = (apConfigs ?? []).map((ap) => ({
      section: ap.section,
      config: { ssid: ap.ssid, encryption: ap.encryption, key: ap.key },
    }));
    const previousRepeaterOptions = repeaterOpts?.allow_ap_on_sta_radio ?? null;
    const appliedSections: string[] = [];

    const rollback = async (): Promise<string[]> => {
      const failures: string[] = [];
      // Best effort, in reverse order of application. There is no server-side
      // batch rollback endpoint for the wizard, so each step is reverted with
      // the same API it was applied with.
      for (let i = appliedSections.length - 1; i >= 0; i -= 1) {
        const applied = appliedSections[i];
        const previous = previousApConfigs.find((c) => c.section === applied);
        if (!previous) continue;
        try {
          await setAPMutation.mutateAsync({ section: applied, config: previous.config });
        } catch {
          failures.push(`restore AP "${applied}"`);
        }
      }
      if (previousRepeaterOptions !== null) {
        try {
          await setRepeaterOptsMutation.mutateAsync({
            allow_ap_on_sta_radio: previousRepeaterOptions,
          });
        } catch {
          failures.push('restore repeater options');
        }
      }
      if (previousMode && previousMode !== 'repeater') {
        try {
          await modeMutation.mutateAsync(previousMode);
        } catch {
          failures.push(`restore WiFi mode "${previousMode}"`);
        }
      }
      return failures;
    };

    /**
     * Runs one step of the sequence. A mode switch is a real uci apply +
     * confirm + rollback window on the device, so the sequence deliberately
     * performs it exactly once.
     */
    const step = async (label: string, run: () => Promise<unknown>) => {
      try {
        return await run();
      } catch (err) {
        setFailedStep(label);
        const rollbackFailures = await rollback();
        const detail = err instanceof Error ? err.message : 'Setup failed';
        const suffix = rollbackFailures.length
          ? ` Partial rollback — these steps could not be restored: ${rollbackFailures.join(', ')}.`
          : ' The previous WiFi configuration was restored.';
        throw new Error(`Step ${label} failed: ${detail}.${suffix}`, { cause: err });
      }
    };

    try {
      await step('repeater options', () =>
        setRepeaterOptsMutation.mutateAsync({
          allow_ap_on_sta_radio: allowApOnStaRadio,
        }),
      );
      await step('repeater mode', () => modeMutation.mutateAsync('repeater'));
      await step('upstream connection', () =>
        connectMutation.mutateAsync({
          ssid: upstream.ssid,
          password: upstream.password,
          encryption: upstream.encryption,
          band: selectedNetwork?.band,
        }),
      );

      if (apConfigs && apConfigs.length > 0) {
        for (const ap of apConfigs) {
          let ssid = effectiveAPSSID;
          let enc = effectiveAPEncryption;
          let key = effectiveAPKey;
          if (!apConfig.sameAsUpstream && apConfig.separateBandConfig) {
            const pb = apConfig.perBand[ap.section];
            if (pb) {
              ssid = pb.ssid;
              enc = pb.encryption;
              key = pb.encryption === 'none' ? '' : pb.key;
            }
          }
          await step(`AP "${ap.section}"`, async () => {
            await setAPMutation.mutateAsync({
              section: ap.section,
              config: { ssid, encryption: enc, key },
            });
            appliedSections.push(ap.section);
          });
        }
      }

      setDone(true);
    } catch (err) {
      setApplyError(err instanceof Error ? err.message : 'Setup failed');
    } finally {
      setApplying(false);
    }
  }, [
    setRepeaterOptsMutation,
    allowApOnStaRadio,
    modeMutation,
    connectMutation,
    apConfigs,
    setAPMutation,
    repeaterOpts,
    wifiConn,
    upstream,
    selectedNetwork,
    effectiveAPSSID,
    effectiveAPEncryption,
    effectiveAPKey,
    apConfig.sameAsUpstream,
    apConfig.separateBandConfig,
    apConfig.perBand,
  ]);

  const needsPassword = selectedNetwork?.encryption !== 'none';
  const canProceedUpstream =
    selectedNetwork != null && (!needsPassword || upstream.password.length >= 8);

  const canProceedAP = useMemo(() => {
    if (apConfig.sameAsUpstream) {
      return true;
    }
    if (!apConfig.separateBandConfig) {
      return (
        apConfig.ssid.length > 0 && (apConfig.encryption === 'none' || apConfig.key.length >= 8)
      );
    }
    for (const ap of apConfigs ?? []) {
      const pb = apConfig.perBand[ap.section];
      if (!pb || pb.ssid.trim().length === 0) {
        return false;
      }
      if (pb.encryption !== 'none' && pb.key.length < 8) {
        return false;
      }
    }
    return (apConfigs?.length ?? 0) > 0;
  }, [apConfig, apConfigs]);

  return {
    step,
    setStep,
    selectedNetwork,
    setSelectedNetwork,
    upstream,
    setUpstream,
    apConfig,
    setApConfig,
    allowApOnStaRadio,
    setAllowApOnStaRadio,
    applying,
    applyError,
    failedStep,
    done,
    scanResults,
    scanLoading,
    refetch,
    apConfigs,
    reset,
    handleSelectNetwork,
    handleApply,
    effectiveAPSSID,
    effectiveAPEncryption,
    apSummaryLine,
    needsPassword: !!needsPassword,
    canProceedUpstream,
    canProceedAP,
  };
}
