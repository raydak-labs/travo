import { create } from 'zustand';
import type { Alert } from '@shared/index';

interface AlertStore {
  alerts: Alert[];
  unreadCount: number;
  addAlert: (alert: Alert) => void;
  setAlerts: (alerts: Alert[]) => void;
  markAllRead: () => void;
}

export const useAlertStore = create<AlertStore>((set) => ({
  alerts: [],
  unreadCount: 0,
  addAlert: (alert) =>
    set((state) => {
      // Avoid duplicates
      if (state.alerts.some((a) => a.id === alert.id)) return state;
      return {
        alerts: [alert, ...state.alerts].slice(0, 50),
        unreadCount: state.unreadCount + 1,
      };
    }),
  setAlerts: (alerts) =>
    set((state) => ({
      alerts,
      unreadCount: state.unreadCount, // Don't change unread on history load
    })),
  markAllRead: () => set({ unreadCount: 0 }),
}));
