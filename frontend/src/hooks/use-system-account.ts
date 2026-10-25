/**
 * Account and session hooks: the password change, the alert thresholds, and
 * the setup completion that unlocks the routed application.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient, isTokenRemembered, setToken } from '@/lib/api-client';
import { resetSetupStatusCache } from '@/lib/setup-status';
import { API_ROUTES } from '@shared/index';
import type {
  AlertThresholds,
  ChangePasswordRequest,
  ChangePasswordResponse,
  Status,
} from '@shared/index';

export function useChangePassword() {
  return useMutation({
    mutationFn: async (data: ChangePasswordRequest) => {
      // A password change revokes every session, including this one, and the
      // server issues a replacement token. Keep using the old token here and
      // the very next request fails with 401 — the user is silently logged out
      // right after a successful password change.
      const res = await apiClient.put<ChangePasswordResponse>(API_ROUTES.auth.password, data);
      // Keep the storage the user chose at login: a "don't remember me"
      // session must not become a localStorage token because the token was
      // replaced.
      setToken(res.token, isTokenRemembered());
      return res;
    },
    onSuccess: (res) => {
      const otherSessions = Math.max(0, res.revoked_sessions - 1);
      toast.success('Password changed successfully', {
        description:
          otherSessions > 0
            ? `Signed out of ${otherSessions} other session${otherSessions === 1 ? '' : 's'}.`
            : 'This is the only active session.',
      });
    },
    onError: (error) => {
      toast.error('Failed to change password', { description: error.message });
    },
  });
}

export function useAlertThresholds() {
  return useQuery({
    queryKey: ['system', 'alert-thresholds'],
    queryFn: () => apiClient.get<AlertThresholds>(API_ROUTES.system.alertThresholds),
  });
}

export function useSetAlertThresholds() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (t: AlertThresholds) => apiClient.put<Status>(API_ROUTES.system.alertThresholds, t),
    onSuccess: () => {
      toast.success('Alert thresholds saved');
      void queryClient.invalidateQueries({ queryKey: ['system', 'alert-thresholds'] });
    },
    onError: (error) => {
      toast.error('Failed to save thresholds', { description: error.message });
    },
  });
}

export function useCompleteSetup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.setupComplete),
    onSuccess: () => {
      // The route guard caches the setup answer; finishing setup must not
      // redirect the user straight back to /setup.
      resetSetupStatusCache();
      void queryClient.invalidateQueries({ queryKey: ['system', 'setup-complete'] });
    },
  });
}
