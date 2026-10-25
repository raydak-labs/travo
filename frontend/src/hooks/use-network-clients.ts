/**
 * Client-facing network state: the shared network status query, the connected
 * client list with its mutations, and the interface up/down switch.
 */
import { useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { UseQueryOptions } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useWsSubscribe } from '@/lib/ws-context';
import type { NetworkStatus, Client, SetAliasRequest } from '@shared/index';
import {
  blockClient,
  getBlockedClients,
  getClients,
  getNetworkStatus,
  kickClient,
  setClientAlias,
  setInterfaceState,
  unblockClient,
} from '@/api/endpoints';

export function useNetworkStatus(options?: Partial<UseQueryOptions<NetworkStatus>>) {
  const queryClient = useQueryClient();
  const { subscribe } = useWsSubscribe();

  // The event watcher pushes network_status over WS on interface/client
  // changes; feed it into the cache so every consumer sees live data.
  useEffect(() => {
    return subscribe('network_status', (raw) => {
      queryClient.setQueryData<NetworkStatus>(['network', 'status'], raw as NetworkStatus);
    });
  }, [subscribe, queryClient]);

  return useQuery({
    queryKey: ['network', 'status'],
    queryFn: ({ signal }) => getNetworkStatus({ signal }),
    ...options,
  });
}

/**
 * The connected-client list rides along in `network_status`, which the
 * WebSocket already pushes on interface and client changes. Seeding this cache
 * from those pushes keeps the Clients page live instead of stale until the
 * window regains focus.
 */
export function useClients() {
  const queryClient = useQueryClient();
  const { subscribe } = useWsSubscribe();

  useEffect(() => {
    return subscribe('network_status', (raw) => {
      const status = raw as NetworkStatus;
      if (status.clients) {
        queryClient.setQueryData<Client[]>(['network', 'clients'], [...status.clients]);
      }
    });
  }, [subscribe, queryClient]);

  return useQuery({
    queryKey: ['network', 'clients'],
    queryFn: ({ signal }) => getClients({ signal }),
  });
}

export function useSetClientAlias() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: SetAliasRequest) => setClientAlias(data),
    onSuccess: () => {
      toast.success('Client alias updated');
      void queryClient.invalidateQueries({ queryKey: ['network', 'clients'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'status'] });
    },
    onError: (error) => {
      toast.error('Failed to update alias', { description: error.message });
    },
  });
}

export function useKickClient() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) => kickClient({ mac }),
    onSuccess: () => {
      toast.success('Client disconnected');
      // Without this the kicked device stays in the table until the next
      // window focus refetch.
      void queryClient.invalidateQueries({ queryKey: ['network', 'clients'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'status'] });
    },
    onError: (error) => {
      toast.error('Failed to kick client', { description: error.message });
    },
  });
}

export function useBlockClient() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) => blockClient({ mac }),
    onSuccess: () => {
      toast.success('Client blocked');
      void queryClient.invalidateQueries({ queryKey: ['network', 'blockedClients'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'clients'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'status'] });
    },
    onError: (error) => {
      toast.error('Failed to block client', { description: error.message });
    },
  });
}

export function useUnblockClient() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) => unblockClient({ mac }),
    onSuccess: () => {
      toast.success('Client unblocked');
      void queryClient.invalidateQueries({ queryKey: ['network', 'blockedClients'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'clients'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'status'] });
    },
    onError: (error) => {
      toast.error('Failed to unblock client', { description: error.message });
    },
  });
}

export function useBlockedClients() {
  return useQuery({
    queryKey: ['network', 'blockedClients'],
    queryFn: ({ signal }) => getBlockedClients({ signal }),
  });
}

export function useSetInterfaceState() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, up }: { name: string; up: boolean }) =>
      setInterfaceState(name, {
        up,
      }),
    onSuccess: (_data, variables) => {
      toast.success(`Interface ${variables.name} ${variables.up ? 'brought up' : 'brought down'}`);
      void queryClient.invalidateQueries({ queryKey: ['network', 'status'] });
    },
    onError: (error) => {
      toast.error('Failed to change interface state', { description: error.message });
    },
  });
}
