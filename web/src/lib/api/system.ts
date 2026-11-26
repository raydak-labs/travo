import { client } from './client';
import { getSystemBackup, getSystemInfo } from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

/** Resolves with the system body; rejects on transport, HTTP or shape errors so callers never see "absent". */
export async function fetchSystemInfo(signal?: AbortSignal, requestClient: Client = client) {
  const { data, error, response } = await getSystemInfo({ signal, client: requestClient });
  if (data) return data;
  const detail = (error as { error?: string; detail?: string; title?: string } | undefined) ?? {};
  throw new APIRequestError(
    detail.error ?? detail.detail ?? detail.title ?? `HTTP ${response?.status ?? 'error'}`,
    response?.status,
  );
}

export async function downloadSystemBackup() {
  const { data, response } = await getSystemBackup({ parseAs: 'blob' });
  if (!response?.ok || !(data instanceof Blob) || data.type !== 'application/gzip' || !data.size) {
    throw new Error('Configuration backup failed');
  }

  const url = URL.createObjectURL(data);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'travo-config.tar.gz';
  try {
    link.click();
  } finally {
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
}
