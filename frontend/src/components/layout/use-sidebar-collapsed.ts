import { useState } from 'react';

const STORAGE_KEY_COLLAPSED = 'otg-sidebar-collapsed';

function loadCollapsed(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(STORAGE_KEY_COLLAPSED) === '1';
  } catch {
    return false;
  }
}

function saveCollapsed(collapsed: boolean): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORAGE_KEY_COLLAPSED, collapsed ? '1' : '0');
  } catch {
    /* ignore */
  }
}

/** Persisted desktop sidebar collapsed (icon rail) state. */
export function useSidebarCollapsed() {
  const [collapsed, setCollapsedState] = useState(loadCollapsed);

  const setCollapsed = (value: boolean | ((prev: boolean) => boolean)) => {
    setCollapsedState((prev) => {
      const next = typeof value === 'function' ? value(prev) : value;
      saveCollapsed(next);
      return next;
    });
  };

  return [collapsed, setCollapsed] as const;
}
