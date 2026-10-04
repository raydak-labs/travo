import { beforeEach, describe, expect, it } from 'vitest';
import { useAlertStore } from '@/stores/alert-store';
import type { Alert } from '@shared/index';

function alert(id: string, message = `alert ${id}`): Alert {
  return {
    id,
    message,
    severity: 'info',
    type: 'system',
    timestamp: 1_800_000_000_000,
  } as Alert;
}

describe('alert store merge', () => {
  beforeEach(() => {
    useAlertStore.setState({ alerts: [], unreadCount: 0 });
  });

  it('folds fetched history in without discarding live alerts', () => {
    // A live alert arrives over the socket.
    useAlertStore.getState().addAlert(alert('live'));

    // A poll response, requested *before* that alert existed, resolves.
    useAlertStore.getState().mergeAlerts([alert('old-1'), alert('old-2')]);

    const ids = useAlertStore.getState().alerts.map((a) => a.id);
    expect(ids).toContain('live');
    expect(ids).toEqual(expect.arrayContaining(['old-1', 'old-2']));
  });

  it('dedupes an alert that is in both the live list and the response', () => {
    useAlertStore.getState().addAlert(alert('live'));
    useAlertStore.getState().mergeAlerts([alert('live')]);

    const ids = useAlertStore.getState().alerts.map((a) => a.id);
    expect(ids.filter((id) => id === 'live')).toHaveLength(1);
  });

  it('does not disturb the unread count when history is merged', () => {
    useAlertStore.getState().addAlert(alert('live'));
    expect(useAlertStore.getState().unreadCount).toBe(1);

    useAlertStore.getState().mergeAlerts([alert('old-1')]);
    expect(useAlertStore.getState().unreadCount).toBe(1);
  });

  it('is a no-op when the response contains nothing new', () => {
    useAlertStore.getState().addAlert(alert('live'));
    const before = useAlertStore.getState().alerts;
    useAlertStore.getState().mergeAlerts([]);
    expect(useAlertStore.getState().alerts).toBe(before);
  });
});
