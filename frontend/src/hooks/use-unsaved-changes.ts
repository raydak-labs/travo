import { useEffect } from 'react';

const DEFAULT_MESSAGE = 'You have unsaved changes. Leave this page?';

/**
 * Warns before a page unload discards in-progress edits.
 *
 * There was no guard anywhere in the app, so typing a new Wi-Fi password and
 * navigating away lost it silently.
 */
export function useUnsavedChanges(isDirty: boolean, message: string = DEFAULT_MESSAGE): void {
  useEffect(() => {
    if (!isDirty) return;
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      // Browsers show their own generic copy; this assignment is what triggers it.
      event.returnValue = message;
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [isDirty, message]);
}
