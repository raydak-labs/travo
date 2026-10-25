import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  getAdGuardConfig,
  getAdGuardDNS,
  installService,
  listServices,
  removeService,
  setAdGuardConfig,
  setAdGuardDNS,
  setAdGuardPassword,
  setServiceAutoStart,
  startService,
  stopService,
} from '@/api/endpoints';

export function useServices() {
  return useQuery({
    queryKey: ['services'],
    queryFn: () => listServices(),
  });
}

export function useInstallService() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => installService(id),
    onSuccess: (_data, id) => {
      toast.success(`Service "${id}" installed`);
      void queryClient.invalidateQueries({ queryKey: ['services'] });
    },
    onError: (error, id) => {
      toast.error(`Failed to install "${id}"`, { description: error.message });
    },
  });
}

export function useRemoveService() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => removeService(id),
    onSuccess: (_data, id) => {
      toast.success(`Service "${id}" removed`);
      void queryClient.invalidateQueries({ queryKey: ['services'] });
    },
    onError: (error, id) => {
      toast.error(`Failed to remove "${id}"`, { description: error.message });
    },
  });
}

export function useStartService() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => startService(id),
    onSuccess: (_data, id) => {
      toast.success(`Service "${id}" started`);
      void queryClient.invalidateQueries({ queryKey: ['services'] });
    },
    onError: (error, id) => {
      toast.error(`Failed to start "${id}"`, { description: error.message });
    },
  });
}

export function useStopService() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => stopService(id),
    onSuccess: (_data, id) => {
      toast.success(`Service "${id}" stopped`);
      void queryClient.invalidateQueries({ queryKey: ['services'] });
    },
    onError: (error, id) => {
      toast.error(`Failed to stop "${id}"`, { description: error.message });
    },
  });
}

export function useSetAutoStart() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      setServiceAutoStart(id, {
        enabled,
      }),
    onSuccess: () => {
      toast.success('Auto-start setting updated');
      void queryClient.invalidateQueries({ queryKey: ['services'] });
    },
    onError: (error) => {
      toast.error('Failed to update auto-start', { description: error.message });
    },
  });
}

export function useAdGuardDNS() {
  return useQuery({
    queryKey: ['adguard-dns'],
    queryFn: () => getAdGuardDNS(),
  });
}

export function useSetAdGuardDNS() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => setAdGuardDNS({ enabled }),
    onSuccess: (_data, enabled) => {
      toast.success(enabled ? 'AdGuard DNS enabled' : 'AdGuard DNS disabled');
      void queryClient.invalidateQueries({ queryKey: ['adguard-dns'] });
    },
    onError: (error) => {
      toast.error('Failed to update AdGuard DNS', { description: error.message });
    },
  });
}

export function useAdGuardConfig() {
  return useQuery({
    queryKey: ['adguard-config'],
    queryFn: () => getAdGuardConfig(),
    enabled: false, // Only fetch on demand
  });
}

export function useSetAdGuardConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (content: string) => setAdGuardConfig({ content }),
    onSuccess: () => {
      toast.success('AdGuard configuration saved and service restarted');
      void queryClient.invalidateQueries({ queryKey: ['adguard-config'] });
    },
    onError: (error) => {
      toast.error('Failed to save AdGuard config', { description: error.message });
    },
  });
}

export function useChangeAdGuardPassword() {
  return useMutation({
    mutationFn: ({ username, password }: { username?: string; password: string }) =>
      setAdGuardPassword({ username, password }),
    onSuccess: () => {
      toast.success('AdGuard password updated');
    },
    onError: (error) => {
      toast.error('Failed to update AdGuard password', { description: error.message });
    },
  });
}
