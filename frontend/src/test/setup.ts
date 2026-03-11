import '@testing-library/jest-dom/vitest';
import { server } from '@/mocks/server';
import { beforeAll, afterEach, afterAll } from 'vitest';

// Node 22+ ships a built-in localStorage that is not jsdom-compatible.
// Replace it with an in-memory shim so tests behave like a real browser.
function createStorageShim(): Storage {
  let store: Record<string, string> = {};
  return {
    getItem(key: string) {
      return key in store ? store[key] : null;
    },
    setItem(key: string, value: string) {
      store[key] = String(value);
    },
    removeItem(key: string) {
      delete store[key];
    },
    clear() {
      store = {};
    },
    get length() {
      return Object.keys(store).length;
    },
    key(index: number) {
      return Object.keys(store)[index] ?? null;
    },
  };
}

Object.defineProperty(globalThis, 'localStorage', {
  value: createStorageShim(),
  writable: true,
});
Object.defineProperty(globalThis, 'sessionStorage', {
  value: createStorageShim(),
  writable: true,
});

// Mock window.matchMedia for jsdom
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
