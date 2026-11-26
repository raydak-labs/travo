import type {
  ClientsOutputBody,
  DashboardConnectivity,
  DashboardUplink,
  DashboardUplinks,
  SystemInfoBody,
  TailscaleStatus,
  ThroughputBody,
  TunnelStatus,
} from './api/gen';
import { tailscalePhase } from './api/tailscale';

export type UplinkName = DashboardUplink['name'];
export type SourceState = 'active' | 'standby' | 'down' | 'not_configured' | 'unknown';

export type TopologySource = {
  name: UplinkName;
  state: SourceState;
  /** wwan: SSID, else address; usbtether: tether kind, else address; wan: address. */
  detail: string | null;
  /** Live rate in bits per second; only for an active or standby uplink with a throughput row. */
  rate: { rxBitsPerSecond: number | null; txBitsPerSecond: number | null } | null;
};

export type TopologyModel = {
  /** Always wan, wwan, usbtether in that order. */
  sources: TopologySource[];
  /** Failover enabled; null = unknown. */
  failover: boolean | null;
  router: { hostname: string; model: string } | null;
  /** null = no clients data, never 0. */
  clients: { wifi: number; lan: number; partial: boolean } | null;
  internet: DashboardConnectivity['internet'];
  vpn: 'connected' | 'connecting' | 'off' | 'unknown';
  tailscale: 'connected' | 'off' | 'absent' | 'unknown';
};

/** Each input is the source's last known data; undefined = never loaded, or failed with no prior value. */
export type TopologyInput = {
  uplinks?: DashboardUplinks;
  connectivity?: DashboardConnectivity;
  throughput?: ThroughputBody;
  clients?: ClientsOutputBody;
  resources?: SystemInfoBody;
  tailscale?: TailscaleStatus;
  vpn?: TunnelStatus;
};

const SOURCE_ORDER: readonly UplinkName[] = ['wan', 'wwan', 'usbtether'];

const withoutPrefix = (cidr: string | null) =>
  cidr === null ? null : (cidr.split('/')[0] ?? cidr);

function sourceDetail(uplink: DashboardUplink): string | null {
  const address = withoutPrefix(uplink.ipv4);
  if (uplink.name === 'wwan') return uplink.ssid ?? address;
  if (uplink.name === 'usbtether') return uplink.kind ?? address;
  return address;
}

function sourceState(uplink: DashboardUplink, activeDevice: string | null): SourceState {
  if (!uplink.configured) return 'not_configured';
  // The daemon's `up` already needs an address, so carrier alone is down.
  if (!uplink.up) return 'down';
  return uplink.device !== null && uplink.device === activeDevice ? 'active' : 'standby';
}

const toBits = (bytes: number | null) => (bytes === null ? null : bytes * 8);

function source(
  name: UplinkName,
  input: TopologyInput,
  activeDevice: string | null,
): TopologySource {
  const uplink = input.uplinks?.uplinks.find((u) => u.name === name);
  if (input.uplinks === undefined) return { name, state: 'unknown', detail: null, rate: null };
  // An uplink the daemon does not list has no netifd interface.
  if (uplink === undefined) return { name, state: 'not_configured', detail: null, rate: null };
  const state = sourceState(uplink, activeDevice);
  const row =
    (state === 'active' || state === 'standby') && uplink.device !== null
      ? input.throughput?.interfaces.find((r) => r.name === uplink.device)
      : undefined;
  return {
    name,
    state,
    detail: sourceDetail(uplink),
    rate: row
      ? {
          rxBitsPerSecond: toBits(row.rx_bytes_per_second),
          txBitsPerSecond: toBits(row.tx_bytes_per_second),
        }
      : null,
  };
}

function vpnState(vpn: TunnelStatus | undefined): TopologyModel['vpn'] {
  switch (vpn?.state) {
    case undefined:
      return 'unknown';
    case 'connected':
      return 'connected';
    // Enabled but not proven up: never "off".
    case 'enabled_not_up':
    case 'no_handshake':
      return 'connecting';
    case 'disabled':
    case 'configured':
      return 'off';
  }
}

function tailscaleState(status: TailscaleStatus | undefined): TopologyModel['tailscale'] {
  // A failed read is unknown, never absent.
  if (status === undefined) return 'unknown';
  const phase = tailscalePhase(status);
  if (phase === 'absent' || phase === 'connected') return phase;
  return 'off';
}

/** The model the dashboard topology diagram draws. A missing input only blanks its own part. */
export function topologyModel(input: TopologyInput): TopologyModel {
  const activeDevice = input.connectivity?.active_interface ?? null;
  const clients = input.clients;
  return {
    sources: SOURCE_ORDER.map((name) => source(name, input, activeDevice)),
    failover: input.uplinks?.failover?.enabled ?? null,
    router: input.resources
      ? { hostname: input.resources.hostname, model: input.resources.model }
      : null,
    clients: clients
      ? {
          wifi: clients.clients.filter((c) => c.connection === 'wifi').length,
          lan: clients.clients.filter((c) => c.connection === 'lan').length,
          partial: clients.partial,
        }
      : null,
    internet: input.connectivity?.internet ?? 'unknown',
    vpn: vpnState(input.vpn),
    tailscale: tailscaleState(input.tailscale),
  };
}
