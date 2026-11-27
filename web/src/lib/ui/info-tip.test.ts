import { createRawSnippet } from 'svelte';
import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import InfoTip from './InfoTip.svelte';
import Section from './Section.svelte';
import SettingsRow from './SettingsRow.svelte';

const html = (body: string) => body.replace(/<!--[^>]*-->/g, '');
const trigger = (body: string) => body.match(/<button[^>]*data-slot="popover-trigger"[^>]*>/)?.[0];

describe('InfoTip', () => {
  it('is a labelled button that opens a popover, with the long text kept out of the page', () => {
    const body = html(render(InfoTip, { props: { topic: 'DNS setup', text: 'Long words' } }).body);
    const button = trigger(body);
    expect(button).toContain('type="button"');
    expect(button).toContain('aria-label="More about DNS setup"');
    expect(button).toContain('aria-haspopup="dialog"');
    expect(button).toContain('aria-expanded="false"');
    expect(body).toMatch(/<svg[^>]*lucide-info[^>]*aria-hidden="true"/);
    expect(body).not.toContain('Long words');
  });

  it('sits beside a Section title and a SettingsRow label when info is set', () => {
    const section = html(
      render(Section, { props: { title: 'DNS over HTTPS', description: 'Short.', info: 'Long.' } })
        .body,
    );
    expect(section).toMatch(
      /<h2[^>]*>DNS over HTTPS<\/h2>\s*<button[^>]*More about DNS over HTTPS/,
    );
    expect(section).toContain('Short.');

    const row = html(
      render(SettingsRow, {
        props: {
          label: 'Default DNS',
          info: 'Long.',
          control: createRawSnippet(() => ({ render: () => '<span></span>' })),
        },
      }).body,
    );
    expect(row).toMatch(/Default DNS<\/span>\s*<button[^>]*More about Default DNS/);
    expect(html(render(Section, { props: { title: 'Plain' } }).body)).not.toContain(
      'popover-trigger',
    );
  });
});
