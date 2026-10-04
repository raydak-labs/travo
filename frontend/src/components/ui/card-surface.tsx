import { cva } from 'class-variance-authority';

/**
 * The card-plane surface: one definition shared by `Card` and `PageSection`.
 *
 * The two class strings were byte-identical, which is exactly how duplicates
 * start drifting. Lives in its own module because both consumers need it and
 * `card.tsx` already imports from `page-section.tsx`.
 */
export const cardSurfaceVariants = cva(
  'rounded-lg border border-gray-200 bg-white shadow-sm dark:border-white/10 dark:bg-gray-950',
);
