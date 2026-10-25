import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { checkUpdates, getUpdates } from '@/api/endpoints';

const UPDATES_KEY = ['system', 'updates'] as const;

/**
 * The last update check. The router checks in the background (shortly after
 * boot, then daily), so reading it is cheap; polling picks up that result.
 */
export function useUpdates() {
  return useQuery({
    queryKey: UPDATES_KEY,
    queryFn: ({ signal }) => getUpdates({ signal }),
    refetchInterval: 10 * 60_000,
    staleTime: 60_000,
  });
}

/** Runs a check now: the release list and a package index refresh. */
export function useCheckUpdates() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => checkUpdates(),
    onSuccess: (status) => {
      queryClient.setQueryData(UPDATES_KEY, status);
      if (status.errors.length > 0) {
        toast.warning('Update check incomplete', { description: status.errors.join('; ') });
      }
    },
    onError: (error) => {
      toast.error('Update check failed', { description: error.message });
    },
  });
}

/** Refetches the update status after a package upgrade ended. */
export function useRefreshUpdates() {
  const queryClient = useQueryClient();
  return () => void queryClient.invalidateQueries({ queryKey: UPDATES_KEY });
}
