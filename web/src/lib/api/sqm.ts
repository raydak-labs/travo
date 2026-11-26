import { m } from '../paraglide/messages.js';
import { client } from './client';
import { getSqm, type SqmSettings, setSqm } from './gen';
import type { Client } from './gen/client';
import type { NetworkInterface } from './network-view';
import { APIRequestError } from './request';

export type { SqmSettings };

export const SQM_QDISCS: readonly SqmSettings['qdisc'][] = ['cake', 'fq_codel'];

export const SQM_SCRIPTS: readonly SqmSettings['script'][] = [
  'piece_of_cake.qos',
  'layer_cake.qos',
  'simple.qos',
  'simplest.qos',
  'simplest_tbf.qos',
];

export const SQM_LINKLAYERS: readonly SqmSettings['linklayer'][] = ['none', 'ethernet', 'atm'];

export type InterfaceOption = { value: string; label: string };

export function sqmError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.sqm_err_invalid();
    case 'sqm_not_installed':
      return m.sqm_err_not_installed();
    case 'uci_busy':
      return m.sqm_err_busy();
    case 'transaction_pending':
      return m.vpn_tunnel_busy();
    default:
      return m.sqm_failed();
  }
}

// Uplinks first; always keep the configured interface selectable.
export function sqmInterfaceOptions(
  interfaces: readonly Pick<NetworkInterface, 'name' | 'device' | 'uplink'>[],
  current: string,
): InterfaceOption[] {
  const byDev = new Map<string, { uplink: boolean; names: string[] }>();
  for (const it of interfaces) {
    const dev = it.device?.trim();
    if (!dev || dev === 'lo') continue;
    const entry = byDev.get(dev);
    if (entry) {
      entry.uplink = entry.uplink || it.uplink;
      if (!entry.names.includes(it.name)) entry.names.push(it.name);
    } else {
      byDev.set(dev, { uplink: it.uplink, names: [it.name] });
    }
  }
  const list = [...byDev.entries()]
    .sort(([aDev, a], [bDev, b]) => {
      if (a.uplink !== b.uplink) return a.uplink ? -1 : 1;
      return aDev.localeCompare(bDev);
    })
    .map(([dev, info]) => ({
      value: dev,
      label: `${dev} (${info.names.join(', ')})`,
    }));
  const trimmed = current.trim();
  if (trimmed && !byDev.has(trimmed)) {
    list.unshift({ value: trimmed, label: trimmed });
  }
  return list;
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function fetchSQM(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<SqmSettings> {
  const { data, error, response } = await getSqm({ signal, client: requestClient });
  if (!data) throw new APIRequestError(sqmError(code(error)), response?.status);
  return data;
}

export async function saveSQM(
  settings: SqmSettings,
  requestClient: Client = client,
): Promise<{ settings: SqmSettings } | { error: string }> {
  const body: SqmSettings = {
    enabled: settings.enabled,
    interface: settings.interface.trim(),
    download: Math.trunc(Number(settings.download) || 0),
    upload: Math.trunc(Number(settings.upload) || 0),
    qdisc: settings.qdisc,
    script: settings.script,
    linklayer: settings.linklayer,
    overhead: Math.trunc(Number(settings.overhead) || 0),
  };
  try {
    const { data, error } = await setSqm({ body, client: requestClient });
    return data ? { settings: data } : { error: sqmError(code(error)) };
  } catch {
    return { error: sqmError(undefined) };
  }
}
