import { create } from 'zustand';
import type { Alert } from '@shared/index';

interface AlertStore {
  alerts: Alert[];
  unreadCount: number;
  addAlert: (alert: Alert) => void;
  setAlerts: (alerts: Alert[]) => void;
  /**
   * Folds fetched history into whatever is already held.
   *
   * A poll response describes the router as of the moment the request went out,
   * so it cannot know about alerts that arrived over the WebSocket afterwards.
   * Replacing the list with it silently drops those live alerts while the
   * unread badge keeps counting them.
   */
  mergeAlerts: (alerts: readonly Alert[]) => void;
  markAllRead: () => void;
}

const MAX_ALERTS = 50;

export const useAlertStore = create<AlertStore>((set) => ({
  alerts: [],
  unreadCount: 0,
  addAlert: (alert) =>
    set((state) => {
      // Avoid duplicates
      if (state.alerts.some((a) => a.id === alert.id)) return state;
      return {
        alerts: [alert, ...state.alerts].slice(0, MAX_ALERTS),
        unreadCount: state.unreadCount + 1,
      };
    }),
  setAlerts: (alerts) =>
    set((state) => ({
      alerts,
      unreadCount: state.unreadCount, // Don't change unread on history load
    })),
  mergeAlerts: (fetched: readonly Alert[]) =>
    set((state) => {
      const seen = new Set(state.alerts.map((a) => a.id));
      const incoming = fetched.filter((a: Alert) => !seen.has(a.id));
      if (incoming.length === 0) return state;
      // Server order is newest-first; keeping the existing list in front
      // preserves the live alerts that the response never saw.
      return {
        alerts: [...state.alerts, ...incoming].slice(0, MAX_ALERTS),
        unreadCount: state.unreadCount, // Don't change unread on history load
      };
    }),
  markAllRead: () => set({ unreadCount: 0 }),
}));
