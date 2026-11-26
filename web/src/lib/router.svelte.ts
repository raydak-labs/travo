import { parseHash, type Route } from './router';

let current = $state<Route>(parseHash(location.hash));

window.addEventListener('hashchange', () => {
  current = parseHash(location.hash);
});

export const router = {
  get route(): Route {
    return current;
  },
};
