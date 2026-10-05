/**
 * Indeterminate route-transition bar.
 *
 * `defaultPendingComponent` on the router: shown while an async `beforeLoad`
 * guard runs, so navigation never looks like it simply did nothing.
 */
export function TopProgressBar() {
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label="Loading page"
      className="fixed inset-x-0 top-0 z-50 h-0.5 overflow-hidden bg-blue-100 dark:bg-gray-800"
    >
      <div className="h-full w-1/3 animate-top-progress bg-blue-600 dark:bg-blue-400" />
    </div>
  );
}
