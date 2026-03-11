import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { ServiceInfo } from '@shared/index';

export function useServices() {
  return useQuery({
    queryKey: ['services'],
    queryFn: () => apiClient.get<ServiceInfo[]>(API_ROUTES.services.list),
  });
}

export function useInstallService() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean }>(API_ROUTES.services.install.replace(':id', id)),
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
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean }>(API_ROUTES.services.remove.replace(':id', id)),
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
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean }>(API_ROUTES.services.start.replace(':id', id)),
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
    mutationFn: (id: string) =>
      apiClient.post<{ success: boolean }>(API_ROUTES.services.stop.replace(':id', id)),
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
      apiClient.post<{ status: string }>(API_ROUTES.services.autostart.replace(':id', id), {
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
