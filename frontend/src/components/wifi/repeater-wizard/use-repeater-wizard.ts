import { useState, useCallback, useMemo, useEffect } from 'react';
import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import {
  useWifiScan,
  useWifiConnect,
  useWifiMode,
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
  const [done, setDone] = useState(false);

  const { data: scanResults = [], isLoading: scanLoading, refetch } = useWifiScan(open);
  const { data: apConfigs } = useAPConfigs();
  const { data: repeaterOpts } = useRepeaterOptions(open);
  const connectMutation = useWifiConnect();
  const modeMutation = useWifiMode();
  const setAPMutation = useSetAPConfig();
  const setRepeaterOptsMutation = useSetRepeaterOptions();
  const [repeaterOptsHydrated, setRepeaterOptsHydrated] = useState(false);

  useEffect(() => {
    if (!open) {
      setRepeaterOptsHydrated(false);
      return;
    }
    if (repeaterOpts != null && !repeaterOptsHydrated) {
      setAllowApOnStaRadio(repeaterOpts.allow_ap_on_sta_radio);
      setRepeaterOptsHydrated(true);
    }
  }, [open, repeaterOpts, repeaterOptsHydrated]);

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

    try {
      await setRepeaterOptsMutation.mutateAsync({
        allow_ap_on_sta_radio: allowApOnStaRadio,
      });
      await modeMutation.mutateAsync('repeater');
      await connectMutation.mutateAsync({
        ssid: upstream.ssid,
        password: upstream.password,
        encryption: upstream.encryption,
        band: selectedNetwork?.band,
      });
      await modeMutation.mutateAsync('repeater');

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
          await setAPMutation.mutateAsync({
            section: ap.section,
            config: { ssid, encryption: enc, key },
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
        apConfig.ssid.length > 0 &&
        (apConfig.encryption === 'none' || apConfig.key.length >= 8)
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
