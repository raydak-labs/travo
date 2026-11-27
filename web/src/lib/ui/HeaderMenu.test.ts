import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import HeaderMenu from './HeaderMenu.svelte';

vi.mock('../theme.svelte', () => ({ theme: { value: 'dark', toggle: () => {} } }));

const menu = () =>
  render(HeaderMenu, { props: { open: true, logout: () => {} } }).body.replace(/<!--[^>]*-->/g, '');

describe('HeaderMenu', () => {
  it('links the GitHub repository in a new tab', () => {
    const link = menu().match(
      /<a[^>]*href="https:\/\/github.com\/raydak-labs\/travo"[^>]*>[\s\S]*?<\/a>/,
    )?.[0];
    expect(link).toBeDefined();
    expect(link).toMatch(/\starget="_blank"/);
    expect(link).toMatch(/\srel="noopener noreferrer"/);
    expect(link).toContain('<span>GitHub</span>');
    expect(link).toMatch(/<svg[^>]*lucide-external-link[^>]*aria-hidden="true"/);
  });

  it('places the link between Log out and Restart router', () => {
    const body = menu();
    const github = body.indexOf('GitHub');
    expect(body.indexOf('Log out')).toBeLessThan(github);
    expect(github).toBeLessThan(body.indexOf('Restart router'));
  });
});
