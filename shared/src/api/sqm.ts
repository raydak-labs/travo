export type SQMQdisc = 'cake' | 'fq_codel';

export type SQMScript = 'piece_of_cake.qos' | 'layer_cake.qos' | 'simple.qos';

export interface SQMConfig {
  readonly enabled: boolean;
  readonly interface: string;
  readonly download_kbit: number;
  readonly upload_kbit: number;
  readonly qdisc: SQMQdisc;
  readonly script: SQMScript;
  readonly advanced_hint?: string;
  readonly detected_uci_section?: string;
}

export interface SQMApplyResult {
  readonly ok: boolean;
  readonly output?: string;
  readonly error?: string;
}
