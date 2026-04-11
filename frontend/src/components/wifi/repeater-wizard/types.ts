export type RepeaterWizardStep = 'select-upstream' | 'configure-ap' | 'review';

export interface RepeaterUpstreamConfig {
  ssid: string;
  password: string;
  encryption: string;
}

export interface RepeaterBandFields {
  ssid: string;
  encryption: string;
  key: string;
}

export interface RepeaterApFormConfig {
  ssid: string;
  encryption: string;
  key: string;
  sameAsUpstream: boolean;
  separateBandConfig: boolean;
  perBand: Record<string, RepeaterBandFields>;
}

export interface RepeaterWizardProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}
