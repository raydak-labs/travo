import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import SystemFactoryResetCard from './SystemFactoryResetCard.svelte';

describe('Factory reset card', () => {
  it('starts with the button disabled until the phrase is typed', () => {
    const body = render(SystemFactoryResetCard).body;
    expect(body).toContain('Type "factory reset" to enable the button');
    expect(body).toMatch(
      /<button[^>]*disabled[^>]*>(?:(?!<\/button>)[\s\S])*Erase everything and restart/,
    );
    expect(body).toContain('no root password');
  });
});
