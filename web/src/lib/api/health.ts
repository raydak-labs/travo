import { getHealth } from './gen';
import './client';

/** Resolves with the health body; rejects on transport or HTTP errors so callers never see "absent". */
export async function fetchHealth(signal?: AbortSignal) {
  const { data, error, response } = await getHealth({ signal });
  if (data) return data;
  const detail = (error as { detail?: string; title?: string } | undefined) ?? {};
  throw new Error(detail.detail ?? detail.title ?? `HTTP ${response?.status ?? 'error'}`);
}
