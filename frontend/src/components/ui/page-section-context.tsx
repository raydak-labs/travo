import { createContext, useContext } from 'react';

/**
 * True when rendering inside an expanded/collapsed PageSection body, where
 * `Card` drops its own chrome so the section header reads as the card header.
 */
const PageSectionContext = createContext(false);

export function useInPageSection(): boolean {
  return useContext(PageSectionContext);
}

export { PageSectionContext };
