import type { ReactNode } from 'react';
import { createContext, useContext } from 'react';
import { ChevronDown } from 'lucide-react';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { cn } from '@/lib/cn';

const PageSectionContext = createContext(false);

/** True when rendering inside an expanded/collapsed PageSection body. */
export function useInPageSection(): boolean {
  return useContext(PageSectionContext);
}

type PageSectionProps = {
  title: string;
  defaultOpen?: boolean;
  children: ReactNode;
};

export function PageSection({ title, defaultOpen = false, children }: PageSectionProps) {
  return (
    <Collapsible defaultOpen={defaultOpen}>
      <CollapsibleTrigger
        type="button"
        className={cn(
          'group flex w-full items-center gap-2 rounded-md py-2 text-left text-sm font-medium',
          'text-gray-900 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
          'dark:text-white dark:hover:bg-gray-800',
        )}
      >
        <span className="flex-1 truncate">{title}</span>
        <ChevronDown
          className="h-4 w-4 shrink-0 text-gray-500 transition-transform group-data-[state=open]:rotate-180 dark:text-gray-400"
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <PageSectionContext.Provider value={true}>{children}</PageSectionContext.Provider>
      </CollapsibleContent>
    </Collapsible>
  );
}
