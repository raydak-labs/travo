type Theme = 'light' | 'dark';

let current = $state<Theme>(
  document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark',
);

export const theme = {
  get value(): Theme {
    return current;
  },
  toggle(): void {
    current = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', current);
    try {
      localStorage.setItem('theme', current);
    } catch {
      // storage unavailable: the choice just won't persist
    }
  },
};
