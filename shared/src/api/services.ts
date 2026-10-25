import type { AdGuardConfigBody, ServiceInfo } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type { AdGuardDNSStatus, ServiceInfo } from '../generated/schemas';

/** Service operational state */
export type ServiceState = ServiceInfo['state'];

/** Generated as AdGuardConfigBody. */
export type AdGuardConfig = AdGuardConfigBody;
