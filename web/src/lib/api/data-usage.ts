import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type DataUsageBudget,
  type DataUsageView,
  getDataUsage,
  resetDataBudget,
  setDataBudget,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { DataUsageBudget, DataUsageView };

/** Maps the API's stable error codes to a message; never parses error text. */
export function dataUsageError(code: string | undefined): string {
  switch (code) {
    case 'invalid_request':
      return m.usage_err_invalid();
    case 'data_budget_unreadable':
      return m.usage_err_unreadable();
    default:
      return m.usage_unavailable();
  }
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function fetchDataUsage(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DataUsageView> {
  const { data, error, response } = await getDataUsage({ signal, client: requestClient });
  if (!data) throw new APIRequestError(dataUsageError(code(error)), response?.status);
  return data;
}

type Result = { view: DataUsageView } | { error: string };

export async function saveDataBudget(
  budget: DataUsageBudget,
  requestClient: Client = client,
): Promise<Result> {
  try {
    const { data, error } = await setDataBudget({ body: budget, client: requestClient });
    return data ? { view: data } : { error: dataUsageError(code(error)) };
  } catch {
    return { error: dataUsageError(undefined) };
  }
}

export async function clearDataBudget(requestClient: Client = client): Promise<Result> {
  try {
    const { data, error } = await resetDataBudget({ client: requestClient });
    return data ? { view: data } : { error: dataUsageError(code(error)) };
  } catch {
    return { error: dataUsageError(undefined) };
  }
}
