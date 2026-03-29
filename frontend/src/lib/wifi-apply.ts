import { API_ROUTES } from '@shared/index';
import type { WifiMutationResponse } from '@shared/index';
import { apiClient } from '@/lib/api-client';

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function confirmWifiApply(
  token: string,
  rollbackTimeoutSeconds = 30,
  intervalMs = 1500,
): Promise<void> {
  const deadline = Date.now() + rollbackTimeoutSeconds * 1000;
  let lastError: unknown;

  while (Date.now() <= deadline) {
    try {
      await apiClient.post<{ status: string }>(API_ROUTES.wifi.applyConfirm, { token });
      return;
    } catch (error) {
      lastError = error;
      if (Date.now() + intervalMs > deadline) {
        break;
      }
      await sleep(intervalMs);
    }
  }

  const suffix = lastError instanceof Error && lastError.message ? `: ${lastError.message}` : '';
  throw new Error(`Wireless settings could not be confirmed before rollback timeout${suffix}`);
}

export async function finalizeWifiMutation<T extends WifiMutationResponse>(
  promise: Promise<T>,
): Promise<T> {
  const response = await promise;
  const apply = response.apply;
  if (apply?.pending && apply.token) {
    await confirmWifiApply(apply.token, apply.rollback_timeout_seconds ?? 30);
  }
  return response;
}
