import { useEffect, useState } from 'react';
import {
  NAV_ENTRIES,
  loadSidebarGroupState,
  saveSidebarGroupState,
  isRouteActive,
} from '@/components/layout/nav-config';

/** Collapsible group open state + persistence; auto-expands when a child route is active. */
export function useSidebarGroups(pathname: string) {
  const [groupOpen, setGroupOpen] = useState(loadSidebarGroupState);

  useEffect(() => {
    setGroupOpen((prev) => {
      let next = { ...prev };
      let changed = false;
      for (const entry of NAV_ENTRIES) {
        if (entry.kind === 'group') {
          const childActive = entry.items.some((item) => isRouteActive(item.to, pathname));
          if (childActive && !next[entry.id]) {
            next = { ...next, [entry.id]: true };
            changed = true;
            saveSidebarGroupState(entry.id, true);
          }
        }
      }
      return changed ? next : prev;
    });
  }, [pathname]);

  const setGroup = (id: string, open: boolean) => {
    setGroupOpen((s) => ({ ...s, [id]: open }));
    saveSidebarGroupState(id, open);
  };

  return { groupOpen, setGroup };
}
