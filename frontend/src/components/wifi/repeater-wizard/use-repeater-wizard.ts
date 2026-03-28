import { useState, useCallback } from 'react';
import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import {
  useWifiScan,
  useWifiConnect,
  useWifiMode,
  useAPConfigs,
  useSetAPConfig,
} from '@/hooks/use-wifi';
import type { RepeaterWizardStep, RepeaterUpstreamConfig, RepeaterApFormConfig } from './types';
import { mapScanEncryptionToUci } from './map-encryption';

export function useRepeaterWizard(open: boolean) {
  const [step, setStep] = useState<RepeaterWizardStep>('select-upstream');
  const [selectedNetwork, setSelectedNetwork] = useState<WifiScanResult | null>(null);
  const [upstream, setUpstream] = useState<RepeaterUpstreamConfig>({
    ssid: '',
    password: '',
    encryption: '',
  });
  const [apConfig, setApConfig] = useState<RepeaterApFormConfig>({
    ssid: '',
    encryption: 'psk2',
    key: '',
    sameAsUpstream: true,
  });
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const { data: scanResults = [], isLoading: scanLoading, refetch } = useWifiScan(open);
  const { data: apConfigs } = useAPConfigs();
  const connectMutation = useWifiConnect();
  const modeMutation = useWifiMode();
  const setAPMutation = useSetAPConfig();

  const reset = useCallback(() => {
    setStep('select-upstream');
    setSelectedNetwork(null);
    setUpstream({ ssid: '', password: '', encryption: '' });
    setApConfig({ ssid: '', encryption: 'psk2', key: '', sameAsUpstream: true });
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

  const handleApply = useCallback(async () => {
    setApplying(true);
    setApplyError(null);

    try {
      await modeMutation.mutateAsync('repeater');
      await connectMutation.mutateAsync({
        ssid: upstream.ssid,
        password: upstream.password,
        encryption: upstream.encryption,
        band: selectedNetwork?.band,
      });

      if (apConfigs && apConfigs.length > 0) {
        for (const ap of apConfigs) {
          await setAPMutation.mutateAsync({
            section: ap.section,
            config: {
              ...ap,
              ssid: effectiveAPSSID,
              encryption: effectiveAPEncryption,
              key: effectiveAPKey,
              enabled: true,
            },
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
    modeMutation,
    connectMutation,
    apConfigs,
    setAPMutation,
    upstream,
    selectedNetwork,
    effectiveAPSSID,
    effectiveAPEncryption,
    effectiveAPKey,
  ]);

  const needsPassword = selectedNetwork?.encryption !== 'none';
  const canProceedUpstream =
    selectedNetwork != null && (!needsPassword || upstream.password.length >= 8);
  const canProceedAP =
    apConfig.sameAsUpstream ||
    (apConfig.ssid.length > 0 && (apConfig.encryption === 'none' || apConfig.key.length >= 8));

  return {
    step,
    setStep,
    selectedNetwork,
    setSelectedNetwork,
    upstream,
    setUpstream,
    apConfig,
    setApConfig,
    applying,
    applyError,
    done,
    scanResults,
    scanLoading,
    refetch,
    reset,
    handleSelectNetwork,
    handleApply,
    effectiveAPSSID,
    effectiveAPEncryption,
    needsPassword: !!needsPassword,
    canProceedUpstream,
    canProceedAP,
  };
}
