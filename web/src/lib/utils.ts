export { cn } from 'cn';

// eslint-disable-next-line @typescript-eslint/no-explicit-unknown
export type WithoutChild<T> = T extends { child?: unknown } ? Omit<T, 'child'> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-unknown
export type WithoutChildren<T> = T extends { children?: unknown } ? Omit<T, 'children'> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };
