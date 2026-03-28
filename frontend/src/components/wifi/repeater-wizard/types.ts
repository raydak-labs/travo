export type RepeaterWizardStep = 'select-upstream' | 'configure-ap' | 'review';

export interface RepeaterUpstreamConfig {
  ssid: string;
  password: string;
  encryption: string;
}

export interface RepeaterApFormConfig {
  ssid: string;
  encryption: string;
  key: string;
  sameAsUpstream: boolean;
}

export interface RepeaterWizardProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}
