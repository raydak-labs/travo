/** Service operational state */
export type ServiceState = 'not_installed' | 'installed' | 'running' | 'stopped' | 'error';

/** Service information */
export interface ServiceInfo {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly state: ServiceState;
  readonly version?: string;
  readonly auto_start: boolean;
}

/** AdGuard Home status */
export interface AdGuardStatus {
  readonly enabled: boolean;
  readonly total_queries: number;
  readonly blocked_queries: number;
  readonly block_percentage: number;
  readonly avg_response_ms: number;
}

/** AdGuard DNS forwarding status */
export interface AdGuardDNSStatus {
  readonly enabled: boolean;
  readonly dns_port: number;
}

/** AdGuard Home configuration (raw YAML content) */
export interface AdGuardConfig {
  readonly content: string;
}

/** Type guard for ServiceInfo */
export function isServiceInfo(value: unknown): value is ServiceInfo {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.id === 'string' &&
    typeof v.name === 'string' &&
    typeof v.description === 'string' &&
    typeof v.state === 'string' &&
    typeof v.auto_start === 'boolean'
  );
}
